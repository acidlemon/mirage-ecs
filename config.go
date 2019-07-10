package main

import (
	"encoding/json"
	"log"
	"os"
	"regexp"

	"github.com/fsouza/go-dockerclient"
	config "github.com/kayac/go-config"
)

type Config struct {
	Host      Host       `yaml:"host"`
	Listen    Listen     `yaml:"listen"`
	Docker    DockerCfg  `yaml:"docker"`
	Storage   StorageCfg `yaml:"storage"`
	Parameter Paramters  `yaml:"parameters"`
	ECS       ECSCfg     `yaml:"ecs"`
}

type ECSCfg struct {
	Region                string               `yaml:"region"`
	Cluster               string               `yaml:"cluster"`
	LaunchType            string               `yaml:"launch_type"`
	NetworkConfiguration  NetworkConfiguration `yaml:"network_configuration"`
	DefaultTaskDefinition string               `yaml:"default_task_definition"`
}

type NetworkConfiguration struct {
	AwsVpcConfiguration *AwsVpcConfiguration `yaml:"awsvpc_configuration"`
}

type AwsVpcConfiguration struct {
	AssignPublicIp *string   `yaml:"assign_public_ip"`
	SecurityGroups []*string `yaml:"security_groups"`
	Subnets        []*string `yaml:"subnets"`
}

type Host struct {
	WebApi             string `yaml:"webapi"`
	ReverseProxySuffix string `yaml:"reverse_proxy_suffix"`
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

type DockerCfg struct {
	Endpoint     string             `yaml:"endpoint"`
	DefaultImage string             `yaml:"default_image"`
	HostConfig   *docker.HostConfig `yaml:"host_config"` // TODO depending docker.HostConfig is so risky?

}

type StorageCfg struct {
	DataDir string `yaml:"datadir"`
	HtmlDir string `yaml:"htmldir"`
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
		Docker: DockerCfg{
			Endpoint:     "unix:///var/run/docker.sock",
			DefaultImage: "",
		},
		Storage: StorageCfg{
			DataDir: "./data",
			HtmlDir: "./html",
		},
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
	json.NewEncoder(os.Stdout).Encode(cfg)

	return cfg
}
