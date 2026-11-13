package mirageecs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	ttlcache "github.com/ReneKroon/ttlcache/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// route53API is the subset of the Route53 client used by mirage-ecs.
type route53API interface {
	GetHostedZone(ctx context.Context, params *route53.GetHostedZoneInput, optFns ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error)
	ListResourceRecordSets(ctx context.Context, params *route53.ListResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error)
	ChangeResourceRecordSets(ctx context.Context, params *route53.ChangeResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ChangeResourceRecordSetsOutput, error)
}

type Route53 struct {
	svc          route53API
	changes      []*route53Change
	hostedZoneID *string
	zoneName     string
	cache        *ttlcache.Cache
}

type route53Change struct {
	name   string
	value  string
	delete bool
}

func (c *route53Change) String() string {
	return fmt.Sprintf("%s %s %s", c.action(), c.name, c.value)
}

func (c *route53Change) action() string {
	if c.delete {
		return "DELTETE"
	} else {
		return "UPSERT"
	}
}

func NewRoute53(ctx context.Context, cfg *Config) *Route53 {
	return newRoute53(ctx, route53.NewFromConfig(*cfg.awscfg), cfg.Link.HostedZoneID)
}

func newRoute53(ctx context.Context, svc route53API, hostedZoneID string) *Route53 {
	r := &Route53{
		svc: svc,
	}
	if hostedZoneID != "" {
		out, err := svc.GetHostedZone(ctx, &route53.GetHostedZoneInput{
			Id: aws.String(hostedZoneID),
		})
		if err != nil {
			slog.Error(f("failed to get hosted zone %s", err))
			return nil
		}
		r.zoneName = *out.HostedZone.Name
		r.hostedZoneID = out.HostedZone.Id
	}
	r.cache = ttlcache.NewCache()
	r.cache.SetTTL(5 * time.Minute)
	r.cache.SkipTTLExtensionOnHit(true)

	return r
}

func (r *Route53) Add(name, addr string) {
	if r.hostedZoneID == nil {
		return
	}
	change := &route53Change{
		name:  fmt.Sprintf("%s.%s", name, r.zoneName),
		value: addr,
	}
	key := change.String()
	if _, err := r.cache.Get(key); err == nil {
		slog.Debug(f("%s is cached. skip", key))
		return
	}
	r.cache.Set(key, nil)

	slog.Debug(f("route53 change: %s", change.String()))
	r.changes = append(r.changes, change)
}

func (r *Route53) Delete(name string, addr string) {
	if r.hostedZoneID == nil {
		return
	}
	change := &route53Change{
		name:   fmt.Sprintf("%s.%s", name, r.zoneName),
		value:  addr,
		delete: true,
	}
	key := change.String()
	if _, err := r.cache.Get(key); err == nil {
		slog.Debug(f("%s is cached. skip", key))
		return
	}
	r.cache.Set(key, nil)

	slog.Debug(f("route53 change: %s", change.String()))
	r.changes = append(r.changes, change)
}

func (r *Route53) Apply(ctx context.Context) error {
	if r.hostedZoneID == nil || len(r.changes) == 0 {
		return nil
	}
	defer func() {
		// clear changes queue
		r.changes = r.changes[0:0]
	}()

	addes := make(map[string][]*route53Change)
	deletes := make(map[string][]*route53Change)
	for _, c := range r.changes {
		if c.delete {
			deletes[c.name] = append(deletes[c.name], c)
		} else {
			addes[c.name] = append(addes[c.name], c)
		}
	}

	// sum by name
	var changes []types.Change
	for name, cs := range deletes {
		if len(addes[name]) > 0 {
			continue // skip delete when adds exists
		}
		// Route53 rejects the whole change batch when it contains a DELETE
		// for a record set that does not exist (or does not match exactly).
		// So build the DELETE from the record set that currently exists.
		change, err := r.buildDeleteChange(ctx, name, cs)
		if err != nil {
			slog.Warn(f("failed to get the current record set of %s: %s", name, err))
			// retry in the next sync
			for _, c := range cs {
				r.cache.Remove(c.String())
			}
			continue
		}
		if change == nil {
			continue
		}
		changes = append(changes, *change)
		slog.Info(f("route53 change: %v", *change))
	}
	for name, cs := range addes {
		var records []types.ResourceRecord
		for _, c := range cs {
			records = append(records, types.ResourceRecord{Value: &c.value})
		}
		change := types.Change{
			Action: "UPSERT",
			ResourceRecordSet: &types.ResourceRecordSet{
				Name:            aws.String(name),
				ResourceRecords: records,
				TTL:             aws.Int64(60),
				Type:            types.RRTypeA,
			},
		}
		changes = append(changes, change)
		slog.Info(f("route53 change: %v", change))
	}

	if len(changes) == 0 {
		return nil
	}
	_, err := r.svc.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &types.ChangeBatch{
			Changes: changes,
		},
		HostedZoneId: r.hostedZoneID,
	})
	if err != nil {
		return err
	}
	slog.Info(f("route53 ChangeResourceRecordSets complete with %d changes", len(changes)))
	return nil
}

// buildDeleteChange returns a change that removes the addresses of cs from
// the A record set of name that currently exists.
// It returns nil when there is nothing to remove: the record set has already
// been deleted, or it no longer contains the addresses
// (e.g. the subdomain was launched again with a new address).
func (r *Route53) buildDeleteChange(ctx context.Context, name string, cs []*route53Change) (*types.Change, error) {
	current, err := r.currentRecordSet(ctx, name)
	if err != nil {
		return nil, err
	}
	if current == nil {
		slog.Debug(f("record set %s does not exist. skip delete", name))
		return nil, nil
	}

	deleting := make(map[string]bool, len(cs))
	for _, c := range cs {
		deleting[c.value] = true
	}
	var remaining []types.ResourceRecord
	for _, rr := range current.ResourceRecords {
		if rr.Value != nil && deleting[*rr.Value] {
			continue
		}
		remaining = append(remaining, rr)
	}
	switch {
	case len(remaining) == len(current.ResourceRecords):
		slog.Debug(f("record set %s does not contain the addresses to delete. skip delete", name))
		return nil, nil
	case len(remaining) == 0:
		// DELETE must match the existing record set exactly
		return &types.Change{
			Action:            types.ChangeActionDelete,
			ResourceRecordSet: current,
		}, nil
	default:
		// keep the other addresses
		rrset := *current
		rrset.ResourceRecords = remaining
		return &types.Change{
			Action:            types.ChangeActionUpsert,
			ResourceRecordSet: &rrset,
		}, nil
	}
}

// currentRecordSet returns the A record set of name, or nil if it does not exist.
func (r *Route53) currentRecordSet(ctx context.Context, name string) (*types.ResourceRecordSet, error) {
	out, err := r.svc.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId:    r.hostedZoneID,
		StartRecordName: aws.String(name),
		StartRecordType: types.RRTypeA,
		MaxItems:        aws.Int32(1),
	})
	if err != nil {
		return nil, err
	}
	for _, rrset := range out.ResourceRecordSets {
		if rrset.Type == types.RRTypeA && rrset.Name != nil && sameRecordName(*rrset.Name, name) {
			return &rrset, nil
		}
	}
	return nil, nil
}

func sameRecordName(a, b string) bool {
	return strings.EqualFold(strings.TrimSuffix(a, "."), strings.TrimSuffix(b, "."))
}
