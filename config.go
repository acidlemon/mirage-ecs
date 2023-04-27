package main

import (
	"log"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	config "github.com/kayac/go-config"
)

type Config struct {
	Host      Host      `yaml:"host"`
	Listen    Listen    `yaml:"listen"`
	HtmlDir   string    `yaml:"htmldir"`
	Parameter Paramters `yaml:"parameters"`
	ECS       ECSCfg    `yaml:"ecs"`
	Link      Link      `yaml:"link"`

	localMode bool
	session   *session.Session
}

type ECSCfg struct {
	Region                   string                   `yaml:"region"`
	Cluster                  string                   `yaml:"cluster"`
	CapacityProviderStrategy CapacityProviderStrategy `yaml:"capacity_provider_strategy"`
	LaunchType               *string                  `yaml:"launch_type"`
	NetworkConfiguration     *NetworkConfiguration    `yaml:"network_configuration"`
	DefaultTaskDefinition    string                   `yaml:"default_task_definition"`
	EnableExecuteCommand     *bool                    `yaml:"enable_execute_command"`
}

type CapacityProviderStrategy []*CapacityProviderStrategyItem

func (s CapacityProviderStrategy) toSDK() []*ecs.CapacityProviderStrategyItem {
	if len(s) == 0 {
		return nil
	}
	var items []*ecs.CapacityProviderStrategyItem
	for _, item := range s {
		items = append(items, item.toSDK())
	}
	return items
}

type CapacityProviderStrategyItem struct {
	CapacityProvider *string `yaml:"capacity_provider"`
	Weight           *int64  `yaml:"weight"`
	Base             *int64  `yaml:"base"`
}

func (i CapacityProviderStrategyItem) toSDK() *ecs.CapacityProviderStrategyItem {
	return &ecs.CapacityProviderStrategyItem{
		CapacityProvider: i.CapacityProvider,
		Weight:           i.Weight,
		Base:             i.Base,
	}
}

type NetworkConfiguration struct {
	AwsVpcConfiguration *AwsVpcConfiguration `yaml:"awsvpc_configuration"`
}

func (c *NetworkConfiguration) toSDK() *ecs.NetworkConfiguration {
	return &ecs.NetworkConfiguration{
		AwsvpcConfiguration: c.AwsVpcConfiguration.toSDK(),
	}
}

type AwsVpcConfiguration struct {
	AssignPublicIp *string   `yaml:"assign_public_ip"`
	SecurityGroups []*string `yaml:"security_groups"`
	Subnets        []*string `yaml:"subnets"`
}

func (c *AwsVpcConfiguration) toSDK() *ecs.AwsVpcConfiguration {
	return &ecs.AwsVpcConfiguration{
		AssignPublicIp: c.AssignPublicIp,
		Subnets:        c.Subnets,
		SecurityGroups: c.SecurityGroups,
	}
}

type Host struct {
	WebApi             string `yaml:"webapi"`
	ReverseProxySuffix string `yaml:"reverse_proxy_suffix"`
}

type Link struct {
	HostedZoneID           string   `yaml:"hosted_zone_id"`
	DefaultTaskDefinitions []string `yaml:"default_task_definitions"`
}

type Listen struct {
	ForeignAddress string    `yaml:"foreign_address"`
	HTTP           []PortMap `yaml:"http"`
	HTTPS          []PortMap `yaml:"https"`
}

type PortMap struct {
	ListenPort int `yaml:"listen"`
	TargetPort int `yaml:"target"`
}

type Parameter struct {
	Name     string `yaml:"name"`
	Env      string `yaml:"env"`
	Rule     string `yaml:"rule"`
	Required bool   `yaml:"required"`
	Regexp   regexp.Regexp
}

type Paramters []*Parameter

func NewConfig(path string) *Config {
	log.Printf("[info] loading config file: %s", path)
	// default config
	cfg := &Config{
		Host: Host{
			WebApi:             "localhost",
			ReverseProxySuffix: ".dev.example.net",
		},
		Listen: Listen{
			ForeignAddress: "127.0.0.1",
			HTTP:           []PortMap{},
			HTTPS:          []PortMap{},
		},
		HtmlDir: "./html",
	}

	err := config.LoadWithEnv(&cfg, path)
	if err != nil {
		log.Fatalf("cannot load config: %s: %s", path, err)
	}

	for _, v := range cfg.Parameter {
		if v.Rule != "" {
			paramRegex := regexp.MustCompile(v.Rule)
			v.Regexp = *paramRegex
		}
	}

	cfg.session = session.Must(session.NewSession(
		&aws.Config{Region: aws.String(cfg.ECS.Region)},
	))

	return cfg
}
