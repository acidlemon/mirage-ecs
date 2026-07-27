package mirageecs_test

import (
	"testing"
	"time"

	mirageecs "github.com/acidlemon/mirage-ecs/v2"
	"github.com/kayac/go-config"
)

func TestPurgeConfig(t *testing.T) {
	cfg := mirageecs.Config{}
	err := config.LoadWithEnvBytes(&cfg, []byte(`
purge:
  schedule: "*/3 * * * ? *" # every 3 minutes
  request:
    duration: "300" # 5 minutes
    excludes:
      - "test"
      - "test2"
    exclude_tags:
      - "DontPurge:true"
    exclude_regexp: "te.t"
`))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Purge.Validate(); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2024, 11, 7, 11, 22, 33, 0, time.UTC)
	next := cfg.Purge.Cron.Next(now)
	if next != time.Date(2024, 11, 7, 11, 24, 0, 0, time.UTC) {
		t.Errorf("unexpected next time: %s", next)
	}
	if cfg.Purge.PurgeParams.Duration != time.Second*300 {
		t.Errorf("unexpected duration: %d", cfg.Purge.PurgeParams.Duration)
	}
	if len(cfg.Purge.PurgeParams.Excludes) != 2 {
		t.Errorf("unexpected excludes: %v", cfg.Purge.PurgeParams.Excludes)
	}
	if len(cfg.Purge.PurgeParams.ExcludeTags) != 1 {
		t.Errorf("unexpected exclude_tags: %v", cfg.Purge.PurgeParams.ExcludeTags)
	}
	if !cfg.Purge.PurgeParams.ExcludeRegexp.MatchString("test") {
		t.Errorf("unexpected exclude_regexp: %v", cfg.Purge.PurgeParams.ExcludeRegexp)
	}
}
