package mirageecs_test

import (
	"regexp"
	"testing"

	mirageecs "github.com/acidlemon/mirage-ecs/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	config "github.com/kayac/go-config"
)

func TestRecoverConfig(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect *mirageecs.Recover
	}{
		{
			name:   "nil",
			input:  ``,
			expect: nil,
		},
		{
			name: "nil",
			input: `
recover:
`,
			expect: nil,
		},
		{
			name: "default",
			input: `
recover:
  enable: true
`,
			expect: &mirageecs.Recover{
				Enable: true,
				Parameter: &mirageecs.Parameter{
					Name:     "Relaunch",
					Env:      "RELAUNCH",
					Default:  "on",
					Required: false,
					Internal: true,
				},
				HookStoppedReasons: []string{
					"ECS is performing maintenance on the underlying infrastructure hosting the task",
				},
			},
		},
		{
			name: "fill",
			input: `
recover:
  enable: true
  parameter:
    name: xxx
    env: XXX
    default: 1
    required: false
  hook_stopped_reasons:
    - "hoge hoge"
    - "fuga fuga"
`,
			expect: &mirageecs.Recover{
				Enable: true,
				Parameter: &mirageecs.Parameter{
					Name:     "xxx",
					Env:      "XXX",
					Default:  "1",
					Required: false,
					Internal: true,
				},
				HookStoppedReasons: []string{
					"hoge hoge",
					"fuga fuga",
				},
			},
		},
	}
	opt := cmpopts.IgnoreUnexported(regexp.Regexp{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mirageecs.Config{}
			err := config.LoadWithEnvBytes(&cfg, []byte(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(cfg.Recover, tt.expect, opt); diff != "" {
				t.Errorf("Mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
