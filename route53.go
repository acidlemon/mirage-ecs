package mirageecs

import (
	"context"
	"fmt"
	"log"
	"time"

	ttlcache "github.com/ReneKroon/ttlcache/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

type Route53 struct {
	svc          *route53.Client
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
	svc := route53.NewFromConfig(*cfg.awscfg)
	r := &Route53{
		svc: svc,
	}
	if id := cfg.Link.HostedZoneID; id != "" {
		out, err := svc.GetHostedZone(ctx, &route53.GetHostedZoneInput{
			Id: aws.String(id),
		})
		if err != nil {
			log.Println("[error] failed to get hosted zone", err)
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
		log.Printf("[debug] %s is cached. skip", key)
		return
	}
	r.cache.Set(key, nil)

	log.Println("[debug] route53 change:", change.String())
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
		log.Printf("[debug] %s is cached. skip", key)
		return
	}
	r.cache.Set(key, nil)

	log.Println("[debug] route53 change:", change.String())
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
DELETES:
	for name, cs := range deletes {
		var records []types.ResourceRecord
		for _, c := range cs {
			if len(addes[c.name]) > 0 {
				continue DELETES // skip delete when adds exists
			}
			records = append(records, types.ResourceRecord{Value: &c.value})
		}
		change := types.Change{
			Action: "DELETE",
			ResourceRecordSet: &types.ResourceRecordSet{
				Name:            aws.String(name),
				ResourceRecords: records,
				TTL:             aws.Int64(60),
				Type:            types.RRTypeA,
			},
		}
		changes = append(changes, change)
		log.Printf("[info] route53 change: %v", change)
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
		log.Printf("[info] route53 change: %v", change)
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
	log.Printf("[info] route53 ChangeResourceRecordSets complete with %d changes", len(changes))
	return nil
}
