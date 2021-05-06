package main

import (
	"fmt"
	"log"
	"time"

	ttlcache "github.com/ReneKroon/ttlcache/v2"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
)

type Route53 struct {
	svc          *route53.Route53
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
	if c.delete {
		return fmt.Sprintf("delete %s %s", c.name, c.value)
	} else {
		return fmt.Sprintf("add %s %s", c.name, c.value)
	}
}

func NewRoute53(cfg *Config) *Route53 {
	sess := session.Must(session.NewSession(
		&aws.Config{Region: aws.String(cfg.ECS.Region)},
	))
	svc := route53.New(sess)
	r := &Route53{
		svc: svc,
	}
	if id := cfg.Link.HostedZoneID; id != "" {
		out, err := svc.GetHostedZone(&route53.GetHostedZoneInput{
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

func (r *Route53) Apply() error {
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
	var changes []*route53.Change
	for name, cs := range deletes {
		var records []*route53.ResourceRecord
		for _, c := range cs {
			records = append(records, &route53.ResourceRecord{Value: &c.value})
		}
		change := &route53.Change{
			Action: aws.String("DELETE"),
			ResourceRecordSet: &route53.ResourceRecordSet{
				Name:            aws.String(name),
				ResourceRecords: records,
				TTL:             aws.Int64(60),
				Type:            aws.String("A"),
			},
		}
		changes = append(changes, change)
		log.Printf("[info] route53 change: %s", change.String())
	}
	for name, cs := range addes {
		var records []*route53.ResourceRecord
		for _, c := range cs {
			records = append(records, &route53.ResourceRecord{Value: &c.value})
		}
		change := &route53.Change{
			Action: aws.String("UPSERT"),
			ResourceRecordSet: &route53.ResourceRecordSet{
				Name:            aws.String(name),
				ResourceRecords: records,
				TTL:             aws.Int64(60),
				Type:            aws.String("A"),
			},
		}
		changes = append(changes, change)
		log.Printf("[info] route53 change: %s", change.String())
	}

	_, err := r.svc.ChangeResourceRecordSets(&route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
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
