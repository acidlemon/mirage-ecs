package mirageecs_test

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	mirageecs "github.com/acidlemon/mirage-ecs"
)

func TestToECSKeyValuePairsAndTags(t *testing.T) {
	tests := []struct {
		name         string
		taskParam    mirageecs.TaskParameter
		configParams mirageecs.Parameters
		subdomain    string
		expectedKVP  []types.KeyValuePair
		expectedTags []types.Tag
		expectedEnv  map[string]string
	}{
		{
			name: "Basic Test",
			taskParam: mirageecs.TaskParameter{
				"Param1": "Value1",
				"Param2": "Value2",
			},
			configParams: mirageecs.Parameters{
				&mirageecs.Parameter{Name: "Param1", Env: "ENV1"},
				&mirageecs.Parameter{Name: "Param2", Env: "ENV2"},
				&mirageecs.Parameter{Name: "Param3", Env: "ENV3"},
			},
			subdomain: "testsubdomain",
			expectedKVP: []types.KeyValuePair{
				{Name: aws.String("SUBDOMAIN"), Value: aws.String("dGVzdHN1YmRvbWFpbg==")},
				{Name: aws.String("SUBDOMAINRAW"), Value: aws.String("testsubdomain")},
				{Name: aws.String("ENV1"), Value: aws.String("Value1")},
				{Name: aws.String("ENV2"), Value: aws.String("Value2")},
			},
			expectedTags: []types.Tag{
				{Key: aws.String("Subdomain"), Value: aws.String("dGVzdHN1YmRvbWFpbg==")},
				{Key: aws.String("SubdomainRaw"), Value: aws.String("testsubdomain")},
				{Key: aws.String("ManagedBy"), Value: aws.String(mirageecs.TagValueMirage)},
				{Key: aws.String("Param1"), Value: aws.String("Value1")},
				{Key: aws.String("Param2"), Value: aws.String("Value2")},
			},
			expectedEnv: map[string]string{
				"SUBDOMAIN":    "dGVzdHN1YmRvbWFpbg==",
				"SUBDOMAINRAW": "testsubdomain",
				"ENV1":         "Value1",
				"ENV2":         "Value2",
			},
		},
	}

	opt := cmpopts.IgnoreUnexported(types.KeyValuePair{}, types.Tag{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kvpResult := tt.taskParam.ToECSKeyValuePairs(tt.subdomain, tt.configParams)
			if diff := cmp.Diff(kvpResult, tt.expectedKVP, opt); diff != "" {
				t.Errorf("Mismatch in KeyValuePairs (-got +want):\n%s", diff)
			}
			tagsResult := tt.taskParam.ToECSTags(tt.subdomain, tt.configParams)
			if diff := cmp.Diff(tagsResult, tt.expectedTags, opt); diff != "" {
				t.Errorf("Mismatch in Tags (-got +want):\n%s", diff)
			}
			envResult := tt.taskParam.ToEnv(tt.subdomain, tt.configParams)
			if diff := cmp.Diff(envResult, tt.expectedEnv, opt); diff != "" {
				t.Errorf("Mismatch in Env (-got +want):\n%s", diff)
			}
		})
	}
}

var purgeTests = []struct {
	name           string
	duration       time.Duration
	excludesMap    map[string]struct{}
	excludeTagsMap map[string]string
	expected       bool
}{
	{
		name:     "young task",
		duration: 10 * time.Minute,
		expected: false,
	},
	{
		name:     "old task",
		duration: 1 * time.Minute,
		expected: true,
	},
	{
		name:     "excluded task",
		duration: 1 * time.Minute,
		excludesMap: map[string]struct{}{
			"test": {},
		},
		expected: false,
	},
	{
		name:     "excluded task not match",
		duration: 1 * time.Minute,
		excludesMap: map[string]struct{}{
			"test2": {},
		},
		expected: true,
	},
	{
		name:     "excluded tag",
		duration: 1 * time.Minute,
		excludeTagsMap: map[string]string{
			"DontPurge": "true",
		},
		expected: false,
	},
	{
		name:     "excluded tag not match",
		duration: 1 * time.Minute,
		excludeTagsMap: map[string]string{
			"xxx": "true",
		},
		expected: true,
	},
}

func TestShouldBePurged(t *testing.T) {
	info := mirageecs.Information{
		ID:         "0123456789abcdef",
		ShortID:    "testshortid",
		SubDomain:  "test",
		GitBranch:  "develop",
		TaskDef:    "dummy",
		IPAddress:  "127.0.0.1",
		Created:    time.Now().Add(-5 * time.Minute),
		LastStatus: "RUNNING",
		PortMap:    map[string]int{"http": 80},
		Env:        map[string]string{"ENV": "test"},
		Tags: []types.Tag{
			{Key: aws.String("Subdomain"), Value: aws.String("test")},
			{Key: aws.String("DontPurge"), Value: aws.String("true")},
		},
	}
	for _, s := range purgeTests {
		t.Run(s.name, func(t *testing.T) {
			if info.ShouldBePurged(s.duration, s.excludesMap, s.excludeTagsMap) != s.expected {
				t.Errorf("Mismatch in ShouldBePurged: %v", s)
			}
		})
	}
}
