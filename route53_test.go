package mirageecs_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	mirageecs "github.com/acidlemon/mirage-ecs/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// mockRoute53 is an in-memory hosted zone that behaves like Route53:
// a change batch is rejected as a whole when it contains a DELETE
// that does not match an existing record set exactly.
type mockRoute53 struct {
	zoneName string
	records  map[string][]string // name -> values
	batches  [][]types.Change
	listErrs []error // errors returned by ListResourceRecordSets, one per call
}

var _ mirageecs.Route53API = (*mockRoute53)(nil)

func newMockRoute53(records map[string][]string) *mockRoute53 {
	if records == nil {
		records = map[string][]string{}
	}
	return &mockRoute53{zoneName: "example.local.", records: records}
}

func (m *mockRoute53) GetHostedZone(ctx context.Context, in *route53.GetHostedZoneInput, _ ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error) {
	return &route53.GetHostedZoneOutput{
		HostedZone: &types.HostedZone{Id: in.Id, Name: aws.String(m.zoneName)},
	}, nil
}

func (m *mockRoute53) ListResourceRecordSets(ctx context.Context, in *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	if len(m.listErrs) > 0 {
		err := m.listErrs[0]
		m.listErrs = m.listErrs[1:]
		return nil, err
	}
	// return the first record set at or after StartRecordName, like Route53
	names := make([]string, 0, len(m.records))
	for name := range m.records {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if name < *in.StartRecordName {
			continue
		}
		return &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []types.ResourceRecordSet{m.recordSet(name)},
		}, nil
	}
	return &route53.ListResourceRecordSetsOutput{}, nil
}

func (m *mockRoute53) recordSet(name string) types.ResourceRecordSet {
	var rrs []types.ResourceRecord
	for _, v := range m.records[name] {
		rrs = append(rrs, types.ResourceRecord{Value: aws.String(v)})
	}
	return types.ResourceRecordSet{
		Name:            aws.String(name),
		Type:            types.RRTypeA,
		TTL:             aws.Int64(60),
		ResourceRecords: rrs,
	}
}

func values(rrset *types.ResourceRecordSet) []string {
	var vs []string
	for _, rr := range rrset.ResourceRecords {
		vs = append(vs, *rr.Value)
	}
	sort.Strings(vs)
	return vs
}

type invalidChangeBatch struct{ msg string }

func (e *invalidChangeBatch) Error() string { return "InvalidChangeBatch: " + e.msg }

func (m *mockRoute53) ChangeResourceRecordSets(ctx context.Context, in *route53.ChangeResourceRecordSetsInput, _ ...func(*route53.Options)) (*route53.ChangeResourceRecordSetsOutput, error) {
	m.batches = append(m.batches, in.ChangeBatch.Changes)
	// validate all changes first: the batch is atomic
	for _, c := range in.ChangeBatch.Changes {
		if c.Action != types.ChangeActionDelete {
			continue
		}
		name := *c.ResourceRecordSet.Name
		current, ok := m.records[name]
		if !ok {
			return nil, &invalidChangeBatch{"Tried to delete resource record set " + name + " but it was not found"}
		}
		sort.Strings(current)
		if strings.Join(current, ",") != strings.Join(values(c.ResourceRecordSet), ",") {
			return nil, &invalidChangeBatch{"Tried to delete resource record set " + name + " but the values provided do not match the current values"}
		}
	}
	for _, c := range in.ChangeBatch.Changes {
		name := *c.ResourceRecordSet.Name
		switch c.Action {
		case types.ChangeActionDelete:
			delete(m.records, name)
		default:
			m.records[name] = values(c.ResourceRecordSet)
		}
	}
	return &route53.ChangeResourceRecordSetsOutput{}, nil
}

func newTestRoute53(t *testing.T, m *mockRoute53) *mirageecs.Route53 {
	t.Helper()
	r := mirageecs.NewRoute53WithClient(context.Background(), m, "Z0000000000")
	if r == nil {
		t.Fatal("failed to create Route53")
	}
	return r
}

func TestRoute53DeleteMissingRecordDoesNotBlockUpsert(t *testing.T) {
	// The record of a stopped task has already been deleted
	// (e.g. by a previous sync or by another mirage-ecs process).
	m := newMockRoute53(nil)
	r := newTestRoute53(t, m)

	r.Delete("app.stopped", "10.0.0.1")
	r.Add("app.running", "10.0.0.2")
	if err := r.Apply(context.Background()); err != nil {
		t.Fatalf("Apply should succeed: %s", err)
	}
	if got := m.records["app.running.example.local."]; len(got) != 1 || got[0] != "10.0.0.2" {
		t.Errorf("app.running should be registered: %v", got)
	}
	for _, c := range m.batches[len(m.batches)-1] {
		if c.Action == types.ChangeActionDelete {
			t.Errorf("DELETE for a missing record should not be sent: %s", *c.ResourceRecordSet.Name)
		}
	}
}

func TestRoute53DeleteExistingRecord(t *testing.T) {
	m := newMockRoute53(map[string][]string{
		"app.stopped.example.local.": {"10.0.0.1"},
	})
	r := newTestRoute53(t, m)

	r.Delete("app.stopped", "10.0.0.1")
	if err := r.Apply(context.Background()); err != nil {
		t.Fatalf("Apply should succeed: %s", err)
	}
	if _, ok := m.records["app.stopped.example.local."]; ok {
		t.Errorf("app.stopped should be deleted")
	}
}

func TestRoute53DeleteSkipsRecordWithNewAddress(t *testing.T) {
	// The subdomain was launched again and the record points to a new address.
	m := newMockRoute53(map[string][]string{
		"app.relaunched.example.local.": {"10.0.0.9"},
	})
	r := newTestRoute53(t, m)

	r.Delete("app.relaunched", "10.0.0.1")
	if err := r.Apply(context.Background()); err != nil {
		t.Fatalf("Apply should succeed: %s", err)
	}
	if got := m.records["app.relaunched.example.local."]; len(got) != 1 || got[0] != "10.0.0.9" {
		t.Errorf("the record with the new address should be kept: %v", got)
	}
	if len(m.batches) != 0 {
		t.Errorf("no change should be sent: %v", m.batches)
	}
}

func TestRoute53DeleteKeepsOtherAddresses(t *testing.T) {
	m := newMockRoute53(map[string][]string{
		"app.multi.example.local.": {"10.0.0.1", "10.0.0.2"},
	})
	r := newTestRoute53(t, m)

	r.Delete("app.multi", "10.0.0.1")
	if err := r.Apply(context.Background()); err != nil {
		t.Fatalf("Apply should succeed: %s", err)
	}
	if got := m.records["app.multi.example.local."]; len(got) != 1 || got[0] != "10.0.0.2" {
		t.Errorf("only 10.0.0.2 should remain: %v", got)
	}
}

func TestRoute53DeleteDoesNotMatchNeighborRecord(t *testing.T) {
	// ListResourceRecordSets returns the next record set when the name does not exist.
	m := newMockRoute53(map[string][]string{
		"app.zzz.example.local.": {"10.0.0.1"},
	})
	r := newTestRoute53(t, m)

	r.Delete("app.aaa", "10.0.0.1")
	if err := r.Apply(context.Background()); err != nil {
		t.Fatalf("Apply should succeed: %s", err)
	}
	if _, ok := m.records["app.zzz.example.local."]; !ok {
		t.Errorf("app.zzz should not be deleted")
	}
}

func TestRoute53DeleteRetriesAfterLookupFailure(t *testing.T) {
	m := newMockRoute53(map[string][]string{
		"app.stopped.example.local.": {"10.0.0.1"},
	})
	m.listErrs = []error{errors.New("lookup failed")}
	r := newTestRoute53(t, m)

	// The failed lookup must not block the UPSERT.
	r.Delete("app.stopped", "10.0.0.1")
	r.Add("app.running", "10.0.0.2")
	if err := r.Apply(context.Background()); err != nil {
		t.Fatalf("Apply should succeed: %s", err)
	}
	if got := m.records["app.running.example.local."]; len(got) != 1 || got[0] != "10.0.0.2" {
		t.Errorf("app.running should be registered: %v", got)
	}
	if _, ok := m.records["app.stopped.example.local."]; !ok {
		t.Errorf("app.stopped should not be deleted yet")
	}

	// The DELETE must be retried in the next sync, without waiting for
	// the cache to expire.
	r.Delete("app.stopped", "10.0.0.1")
	if err := r.Apply(context.Background()); err != nil {
		t.Fatalf("Apply should succeed: %s", err)
	}
	if _, ok := m.records["app.stopped.example.local."]; ok {
		t.Errorf("app.stopped should be deleted by the retry")
	}
}
