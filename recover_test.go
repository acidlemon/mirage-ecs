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

func TestRecoverConfigIsEnable(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *mirageecs.Recover
		expect bool
	}{
		{
			name:   "nil",
			cfg:    nil,
			expect: false,
		},
		{
			name: "Enable=false",
			cfg: &mirageecs.Recover{
				Enable: false,
			},
			expect: false,
		},
		{
			name: "Enable=true",
			cfg: &mirageecs.Recover{
				Enable: true,
			},
			expect: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if g, w := tt.cfg.IsEnable(), tt.expect; g != w {
				t.Errorf("unexpected IsEnable: want %t, but got %t", w, g)
			}
		})
	}
}

func TestRecoverConfigAddOrSkipParameter(t *testing.T) {
	tests := []struct {
		name             string
		configParameters mirageecs.Parameters
		configRecover    *mirageecs.Recover
		expect           mirageecs.Parameters
	}{
		{
			name:             "skip: cfg=nil",
			configParameters: nil,
			configRecover:    nil,
			expect:           nil,
		},
		{
			name:             "skip: Enable=false",
			configParameters: nil,
			configRecover: &mirageecs.Recover{
				Enable: false,
				Parameter: &mirageecs.Parameter{
					Name:     "xxx",
					Env:      "XXX",
					Default:  "on",
					Required: false,
				},
			},
			expect: nil,
		},
		{
			name: "skip: Enable=true duplicate",
			configParameters: mirageecs.Parameters{
				{
					Name:     "xxx",
					Env:      "XXX",
					Default:  "on",
					Required: false,
				},
			},
			configRecover: &mirageecs.Recover{
				Enable: false,
				Parameter: &mirageecs.Parameter{
					Name:     "yyy",
					Env:      "YYY",
					Default:  "1",
					Required: true,
				},
			},
			expect: mirageecs.Parameters{
				{
					Name:     "xxx",
					Env:      "XXX",
					Default:  "on",
					Required: false,
				},
			},
		},
		{
			name:             "add: Enable=true",
			configParameters: nil,
			configRecover: &mirageecs.Recover{
				Enable: true,
				Parameter: &mirageecs.Parameter{
					Name:     "xxx",
					Env:      "XXX",
					Default:  "on",
					Required: false,
				},
			},
			expect: mirageecs.Parameters{
				{
					Name:     "xxx",
					Env:      "XXX",
					Default:  "on",
					Required: false,
				},
			},
		},
	}
	opt := cmpopts.IgnoreUnexported(regexp.Regexp{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.configRecover.AddOrSkipParameter(tt.configParameters)
			if diff := cmp.Diff(got, tt.expect, opt); diff != "" {
				t.Errorf("Mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestRecoverConfigRecoverTaskParameter(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *mirageecs.Recover
		input  mirageecs.TaskParameter
		expect mirageecs.TaskParameter
	}{
		{
			name:   "cfg=nil",
			cfg:    nil,
			input:  nil,
			expect: nil,
		},
		{
			name: "Enable=false",
			cfg: &mirageecs.Recover{
				Enable: false,
			},
			input:  nil,
			expect: nil,
		},
		{
			name: "basic",
			cfg: &mirageecs.Recover{
				Enable: true,
				Parameter: &mirageecs.Parameter{
					Name:    "xxx",
					Default: "on",
				},
			},
			input: nil,
			expect: mirageecs.TaskParameter{
				"xxx": "on",
			},
		},
		{
			name: "overwrite",
			cfg: &mirageecs.Recover{
				Enable: true,
				Parameter: &mirageecs.Parameter{
					Name:    "xxx",
					Default: "yyy",
				},
			},
			input: mirageecs.TaskParameter{
				"xxx": "on",
			},
			expect: mirageecs.TaskParameter{
				"xxx": "yyy",
			},
		},
	}
	opt := cmpopts.IgnoreUnexported(regexp.Regexp{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.RecoverTaskParameter(tt.input)
			if diff := cmp.Diff(got, tt.expect, opt); diff != "" {
				t.Errorf("Mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
