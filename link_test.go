package mirageecs_test

import (
	"testing"

	mirageecs "github.com/acidlemon/mirage-ecs/v2"
)

func TestLinkShouldRegisterRecord(t *testing.T) {
	t.Run("returns false when hosted_zone_id is not set", func(t *testing.T) {
		link := &mirageecs.Link{}
		if mirageecs.LinkShouldRegisterRecord(link, "httpd") {
			t.Errorf("should not register record when hosted_zone_id is not set")
		}
	})

	t.Run("filters by exclude_containers", func(t *testing.T) {
		link := &mirageecs.Link{
			HostedZoneID:      "Z00000000000000000000",
			ExcludeContainers: []string{"log-router", "dd-agent", "sidecar-*"},
		}
		cases := []struct {
			name     string
			expected bool
		}{
			// should register
			{"httpd", true},
			{"app", true},
			{"sidecar", true}, // "sidecar-*" requires a suffix, so "sidecar" itself is not excluded
			// excluded by exact match
			{"log-router", false},
			{"dd-agent", false},
			// excluded by wildcard
			{"sidecar-agent", false},
			{"sidecar-metrics", false},
		}
		for _, c := range cases {
			if got := mirageecs.LinkShouldRegisterRecord(link, c.name); got != c.expected {
				t.Errorf("shouldRegisterRecord(%q) = %v, want %v", c.name, got, c.expected)
			}
		}
	})
}
