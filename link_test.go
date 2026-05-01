package mirageecs_test

import (
	"testing"

	mirageecs "github.com/acidlemon/mirage-ecs/v2"
)

func TestLinkIsExcluded(t *testing.T) {
	t.Run("returns true when hosted_zone_id is not set", func(t *testing.T) {
		link := &mirageecs.Link{}
		if !mirageecs.LinkIsExcluded(link, "httpd") {
			t.Errorf("should be excluded when hosted_zone_id is not set")
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
			// not excluded
			{"httpd", false},
			{"app", false},
			{"sidecar", false}, // "sidecar-*" requires a suffix, so "sidecar" itself is not excluded
			// excluded by exact match
			{"log-router", true},
			{"dd-agent", true},
			// excluded by wildcard
			{"sidecar-agent", true},
			{"sidecar-metrics", true},
		}
		for _, c := range cases {
			if got := mirageecs.LinkIsExcluded(link, c.name); got != c.expected {
				t.Errorf("isExcluded(%q) = %v, want %v", c.name, got, c.expected)
			}
		}
	})
}
