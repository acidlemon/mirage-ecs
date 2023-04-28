package main

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestNewConfig(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Error(err)
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	data := `---
host:
  webapi: localhost
  reverse_proxy_suffix: .dev.example.net
listen:
  foreign_address: 127.0.0.1
  http:
    - listen: 8080
      target: 5000
ecs:
  region: ap-northeast-1
  cluster: test-cluster
  default_task_definition: test-task-definition
  capacity_provider_strategy:
    - capacity_provider: test-strategy
      base: 1
      weight: 1
  enable_execute_command: true
  network_configuration:
    awsvpc_configuration:
      subnets:
        - subnet-aaaa
        - subnet-bbbb
        - subnet-cccc
      security_groups:
        - sg-gggg
      assign_public_ip: ENABLED

storage:
  datadir: ./data
  htmldir: ./html
parameters:
  - name: branch
    env: GIT_BRANCH
    rule: "[0-9a-z-]{32}"
    required: true
  - name: nick
    env: NICK
    rule: "[0-9A-Za-z]{10}"
    required: false

link:
  hosted_zone_id: Z00000000000000000000
  default_task_definitions:
    - test-task-definition
    - test-task-definition-link
`

	if err := ioutil.WriteFile(f.Name(), []byte(data), 0644); err != nil {
		t.Error(err)
	}

	cfg, err := NewConfig(&ConfigParams{Path: f.Name()})
	if err != nil {
		t.Error(err)
	}

	if cfg.Parameter[0].Name != "branch" {
		t.Error("could not parse parameter")
	}

	if cfg.Parameter[1].Env != "NICK" {
		t.Error("could not parse parameter")
	}

	if cfg.Parameter[0].Required != true {
		t.Error("could not parse parameter")
	}

	if cfg.ECS.Region != "ap-northeast-1" {
		t.Error("could not parse region")
	}
	if cfg.ECS.Cluster != "test-cluster" {
		t.Error("could not parse cluster")
	}
	if cfg.ECS.DefaultTaskDefinition != "test-task-definition" {
		t.Error("could not parse default_task_definition")
	}
	provider := cfg.ECS.CapacityProviderStrategy[0]
	if *provider.CapacityProvider != "test-strategy" {
		t.Error("could not parse capacity provider strategy")
	}
	if *provider.Base != 1 {
		t.Error("could not parse capacity provider strategy")
	}
	nc := cfg.ECS.NetworkConfiguration
	if *nc.AwsVpcConfiguration.AssignPublicIp != "ENABLED" {
		t.Error("could not parse network configuration")
	}
	if *nc.AwsVpcConfiguration.SecurityGroups[0] != "sg-gggg" {
		t.Error("could not parse network configuration")
	}
	if *nc.AwsVpcConfiguration.Subnets[0] != "subnet-aaaa" {
		t.Error("could not parse network configuration")
	}
	if !*cfg.ECS.EnableExecuteCommand {
		t.Error("could not parse enable execute command")
	}
	if cfg.Link.HostedZoneID != "Z00000000000000000000" {
		t.Error("could not parse link hosted zone")
	}
	if cfg.Link.DefaultTaskDefinitions[0] != "test-task-definition" {
		t.Error("could not parse link default task definitions")
	}
	if cfg.Link.DefaultTaskDefinitions[1] != "test-task-definition-link" {
		t.Error("could not parse link default task definitions")
	}
}
