package mirageecs

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/google/go-cmp/cmp"
)

func TestToECSKeyValuePairsAndTags(t *testing.T) {
	tests := []struct {
		name         string
		taskParam    TaskParameter
		configParams Parameters
		subdomain    string
		expectedKVP  []*ecs.KeyValuePair
		expectedTags []*ecs.Tag
		expectedEnv  map[string]string
	}{
		{
			name: "Basic Test",
			taskParam: TaskParameter{
				"Param1": "Value1",
				"Param2": "Value2",
			},
			configParams: Parameters{
				&Parameter{Name: "Param1", Env: "ENV1"},
				&Parameter{Name: "Param2", Env: "ENV2"},
				&Parameter{Name: "Param3", Env: "ENV3"},
			},
			subdomain: "testsubdomain",
			expectedKVP: []*ecs.KeyValuePair{
				{Name: aws.String("SUBDOMAIN"), Value: aws.String("dGVzdHN1YmRvbWFpbg==")},
				{Name: aws.String("ENV1"), Value: aws.String("Value1")},
				{Name: aws.String("ENV2"), Value: aws.String("Value2")},
			},
			expectedTags: []*ecs.Tag{
				{Key: aws.String("Subdomain"), Value: aws.String("dGVzdHN1YmRvbWFpbg==")},
				{Key: aws.String("ManagedBy"), Value: aws.String(TagValueMirage)},
				{Key: aws.String("Param1"), Value: aws.String("Value1")},
				{Key: aws.String("Param2"), Value: aws.String("Value2")},
			},
			expectedEnv: map[string]string{
				"SUBDOMAIN": "dGVzdHN1YmRvbWFpbg==",
				"ENV1":      "Value1",
				"ENV2":      "Value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kvpResult := tt.taskParam.ToECSKeyValuePairs(tt.subdomain, tt.configParams)
			if diff := cmp.Diff(kvpResult, tt.expectedKVP); diff != "" {
				t.Errorf("Mismatch in KeyValuePairs (-got +want):\n%s", diff)
			}
			tagsResult := tt.taskParam.ToECSTags(tt.subdomain, tt.configParams)
			if diff := cmp.Diff(tagsResult, tt.expectedTags); diff != "" {
				t.Errorf("Mismatch in Tags (-got +want):\n%s", diff)
			}
			envResult := tt.taskParam.ToEnv(tt.subdomain, tt.configParams)
			if diff := cmp.Diff(envResult, tt.expectedEnv); diff != "" {
				t.Errorf("Mismatch in Env (-got +want):\n%s", diff)
			}
		})
	}
}
