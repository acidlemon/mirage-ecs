package mirageecs_test

import (
	"regexp"
	"strings"
	"testing"

	mirageecs "github.com/acidlemon/mirage-ecs/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kayac/go-config"
)

func TestRecoverConfigValidate(t *testing.T) {
	tests := []struct {
		name   string
		config string
		errMsg string
	}{
		{
			name: "invlaid exclude_parameters",
			config: `
parameters:
  - name: aaa
  - name: bbb

recover:
  exclude_parameters:
    aaa:
    bbb:
    ccc:
`,
			errMsg: "exclude_parameters include invalid parameter: ccc",
		},
		{
			name: "invlaid fixed_parameters",
			config: `
parameters:
  - name: bbb
  - name: ccc

recover:
  fixed_parameters:
    aaa: xxx
    bbb: yyy
    ccc: zzz
`,
			errMsg: "fixed_parameters include invalid parameter: aaa",
		},
		{
			name: "conflict parameters",
			config: `
parameters:
  - name: aaa
  - name: bbb
  - name: ccc
  - name: ddd

recover:
  exclude_parameters:
    aaa:
    bbb:
  fixed_parameters:
    bbb: xxx
    ccc: yyy
`,
			errMsg: "parameters conflicted: bbb",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mirageecs.Config{}
			err := config.LoadWithEnvBytes(&cfg, []byte(tt.config))
			if err != nil {
				t.Fatal(err)
			}
			err = cfg.Recover.Validate(cfg.Parameter)
			if err == nil {
				t.Fatalf("Validate should be return error")
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Fatalf("Unexpected erorr: %v", err)
			}
		})
	}

}

func TestRecoverConfig(t *testing.T) {
	cfg := mirageecs.Config{}
	err := config.LoadWithEnvBytes(&cfg, []byte(`
parameters:
  - name: aaa
  - name: bbb
  - name: ccc
  - name: ddd

recover:
  exclude_parameters:
    aaa:
    bbb:
  fixed_parameters:
    ccc: xxx
    ddd: yyy
`))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Recover.Validate(cfg.Parameter); err != nil {
		t.Fatal(err)
	}
	opt := cmpopts.IgnoreUnexported(regexp.Regexp{})
	if diff := cmp.Diff(cfg.Recover, &mirageecs.Recover{
		ExcludeParameter: map[string]struct{}{
			"aaa": struct{}{},
			"bbb": struct{}{},
		},
		FixedParameter: map[string]string{
			"ccc": "xxx",
			"ddd": "yyy",
		},
		Parameter: cfg.Parameter,
	}, opt); diff != "" {
		t.Errorf("Mismatch (-got +want):\n%s", diff)
	}
}
