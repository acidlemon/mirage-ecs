package mirageecs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsv2Config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	metadata "github.com/brunoscheufler/aws-ecs-metadata-go"
	config "github.com/kayac/go-config"
	"github.com/labstack/echo/v4"
)

var DefaultParameter = &Parameter{
	Name:     "branch",
	Env:      "GIT_BRANCH",
	Rule:     "",
	Required: true,
	Default:  "",
}

type Config struct {
	Host      Host       `yaml:"host"`
	Listen    Listen     `yaml:"listen"`
	Network   Network    `yaml:"network"`
	HtmlDir   string     `yaml:"htmldir"`
	Parameter Parameters `yaml:"parameters"`
	ECS       ECSCfg     `yaml:"ecs"`
	Link      Link       `yaml:"link"`
	Auth      *Auth      `yaml:"auth"`

	localMode bool
	awscfg    *aws.Config
	cleanups  []func() error
}

type ECSCfg struct {
	Region                   string                   `yaml:"region"`
	Cluster                  string                   `yaml:"cluster"`
	CapacityProviderStrategy CapacityProviderStrategy `yaml:"capacity_provider_strategy"`
	LaunchType               *string                  `yaml:"launch_type"`
	NetworkConfiguration     *NetworkConfiguration    `yaml:"network_configuration"`
	DefaultTaskDefinition    string                   `yaml:"default_task_definition"`
	EnableExecuteCommand     *bool                    `yaml:"enable_execute_command"`

	capacityProviderStrategy []types.CapacityProviderStrategyItem `yaml:"-"`
	networkConfiguration     *types.NetworkConfiguration          `yaml:"-"`
}

func (c ECSCfg) String() string {
	m := map[string]interface{}{
		"region":                     c.Region,
		"cluster":                    c.Cluster,
		"capacity_provider_strategy": c.capacityProviderStrategy,
		"launch_type":                c.LaunchType,
		"network_configuration":      c.networkConfiguration,
		"default_task_definition":    c.DefaultTaskDefinition,
		"enable_execute_command":     c.EnableExecuteCommand,
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func (c ECSCfg) validate() error {
	if c.Region == "" {
		return fmt.Errorf("region is required")
	}
	if c.Cluster == "" {
		return fmt.Errorf("cluster is required")
	}
	if c.LaunchType == nil && c.capacityProviderStrategy == nil {
		return fmt.Errorf("launch_type or capacity_provider_strategy is required")
	}
	if c.networkConfiguration == nil {
		return fmt.Errorf("network_configuration is required")
	}
	return nil
}

type CapacityProviderStrategy []*CapacityProviderStrategyItem

func (s CapacityProviderStrategy) toSDK() []types.CapacityProviderStrategyItem {
	if len(s) == 0 {
		return nil
	}
	var items []types.CapacityProviderStrategyItem
	for _, item := range s {
		items = append(items, item.toSDK())
	}
	return items
}

type CapacityProviderStrategyItem struct {
	CapacityProvider *string `yaml:"capacity_provider"`
	Weight           int32   `yaml:"weight"`
	Base             int32   `yaml:"base"`
}

func (i CapacityProviderStrategyItem) toSDK() types.CapacityProviderStrategyItem {
	return types.CapacityProviderStrategyItem{
		CapacityProvider: i.CapacityProvider,
		Weight:           i.Weight,
		Base:             i.Base,
	}
}

type NetworkConfiguration struct {
	AwsVpcConfiguration *AwsVpcConfiguration `yaml:"awsvpc_configuration"`
}

func (c *NetworkConfiguration) toSDK() *types.NetworkConfiguration {
	if c == nil {
		return nil
	}
	return &types.NetworkConfiguration{
		AwsvpcConfiguration: c.AwsVpcConfiguration.toSDK(),
	}
}

type AwsVpcConfiguration struct {
	AssignPublicIp string   `yaml:"assign_public_ip"`
	SecurityGroups []string `yaml:"security_groups"`
	Subnets        []string `yaml:"subnets"`
}

func (c *AwsVpcConfiguration) toSDK() *types.AwsVpcConfiguration {
	return &types.AwsVpcConfiguration{
		AssignPublicIp: types.AssignPublicIp(c.AssignPublicIp),
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
	ForeignAddress string    `yaml:"foreign_address,omitempty"`
	HTTP           []PortMap `yaml:"http,omitempty"`
	HTTPS          []PortMap `yaml:"https,omitempty"`
}

type PortMap struct {
	ListenPort        int  `yaml:"listen"`
	TargetPort        int  `yaml:"target"`
	RequireAuthCookie bool `yaml:"require_auth_cookie"`
}

type Parameter struct {
	Name        string            `yaml:"name"`
	Env         string            `yaml:"env"`
	Rule        string            `yaml:"rule"`
	Required    bool              `yaml:"required"`
	Regexp      regexp.Regexp     `yaml:"-"`
	Default     string            `yaml:"default"`
	Description string            `yaml:"description"`
	Options     []ParameterOption `yaml:"options"`
}

type ParameterOption struct {
	Label string `yaml:"label"`
	Value string `yaml:"value"`
}

type Parameters []*Parameter

type ConfigParams struct {
	Path        string
	Domain      string
	LocalMode   bool
	DefaultPort int
}

type Network struct {
	ProxyTimeout time.Duration `yaml:"proxy_timeout"`
}

const DefaultPort = 80
const DefaultProxyTimeout = 0
const AuthCookieName = "mirage-ecs-auth"
const AuthCookieExpire = 24 * time.Hour

func NewConfig(ctx context.Context, p *ConfigParams) (*Config, error) {
	domain := p.Domain
	if !strings.HasPrefix(domain, ".") {
		domain = "." + domain
	}
	if p.DefaultPort == 0 {
		p.DefaultPort = DefaultPort
	}
	// default config
	cfg := &Config{
		Host: Host{
			WebApi:             "mirage" + domain,
			ReverseProxySuffix: domain,
		},
		Listen: Listen{
			ForeignAddress: "0.0.0.0",
			HTTP: []PortMap{
				{ListenPort: p.DefaultPort, TargetPort: p.DefaultPort},
			},
			HTTPS: nil,
		},
		Network: Network{
			ProxyTimeout: DefaultProxyTimeout,
		},
		HtmlDir: "./html",
		ECS: ECSCfg{
			Region: os.Getenv("AWS_REGION"),
		},
		localMode: p.LocalMode,
		Auth:      nil,
	}

	if awscfg, err := awsv2Config.LoadDefaultConfig(ctx, awsv2Config.WithRegion(cfg.ECS.Region)); err != nil {
		return nil, err
	} else {
		cfg.awscfg = &awscfg
	}

	if p.Path == "" {
		log.Println("[info] no config file specified, using default config with domain suffix: ", domain)
	} else {
		var content []byte
		var err error
		if strings.HasPrefix(p.Path, "s3://") {
			content, err = loadFromS3(ctx, cfg.awscfg, p.Path)
		} else {
			content, err = loadFromFile(p.Path)
		}
		if err != nil {
			return nil, fmt.Errorf("cannot load config: %s: %w", p.Path, err)
		}
		log.Printf("[info] loading config file: %s", p.Path)
		if err := config.LoadWithEnvBytes(&cfg, content); err != nil {
			return nil, fmt.Errorf("cannot load config: %s: %w", p.Path, err)
		}
	}

	addDefaultParameter := true
	for _, v := range cfg.Parameter {
		if v.Name == DefaultParameter.Name {
			addDefaultParameter = false
			break
		}
	}
	if addDefaultParameter {
		cfg.Parameter = append(cfg.Parameter, DefaultParameter)
	}

	for _, v := range cfg.Parameter {
		if v.Rule != "" {
			paramRegex, err := regexp.Compile(v.Rule)
			if err != nil {
				return nil, fmt.Errorf("invalid parameter rule: %s: %w", v.Rule, err)
			}
			v.Regexp = *paramRegex
		}
	}

	if strings.HasPrefix(cfg.HtmlDir, "s3://") {
		if err := cfg.downloadHTMLFromS3(ctx); err != nil {
			return nil, err
		}
	}

	cfg.ECS.capacityProviderStrategy = cfg.ECS.CapacityProviderStrategy.toSDK()
	cfg.ECS.networkConfiguration = cfg.ECS.NetworkConfiguration.toSDK()

	if err := cfg.fillECSDefaults(ctx); err != nil {
		log.Printf("[warn] failed to fill ECS defaults: %s", err)
	}
	return cfg, nil
}

func (c *Config) Cleanup() {
	for _, fn := range c.cleanups {
		if err := fn(); err != nil {
			log.Println("[warn] failed to cleanup", err)
		}
	}
}

func (c *Config) NewTaskRunner() TaskRunner {
	if c.localMode {
		return NewLocalTaskRunner(c)
	} else {
		return NewECSTaskRunner(c)
	}
}

func (c *Config) fillECSDefaults(ctx context.Context) error {
	defer func() {
		if err := c.ECS.validate(); err != nil {
			log.Printf("[error] invalid ECS config: %s", c.ECS)
			log.Printf("[error] ECS config is invalid '%s', so you may not be able to launch ECS tasks", err)
		} else {
			log.Printf("[info] built ECS config: %s", c.ECS)
		}
	}()
	if c.ECS.Region == "" {
		c.ECS.Region = os.Getenv("AWS_REGION")
		log.Printf("[info] AWS_REGION is not set, using region=%s", c.ECS.Region)
	}
	if c.ECS.LaunchType == nil && c.ECS.CapacityProviderStrategy == nil {
		launchType := "FARGATE"
		c.ECS.LaunchType = &launchType
		log.Printf("[info] launch_type and capacity_provider_strategy are not set, using launch_type=%s", *c.ECS.LaunchType)
	}
	if c.ECS.EnableExecuteCommand == nil {
		c.ECS.EnableExecuteCommand = aws.Bool(true)
		log.Printf("[info] enable_execute_command is not set, using enable_execute_command=%t", *c.ECS.EnableExecuteCommand)
	}

	meta, err := metadata.Get(ctx, &http.Client{})
	if err != nil {
		return err
		/*
			for local debugging
			meta = &metadata.TaskMetadataV4{
				Cluster: "your test cluster",
				TaskARN: "your test task arn running on the cluster",
			}
		*/
	}
	log.Printf("[debug] task metadata: %v", meta)
	var cluster, taskArn, service string
	switch m := meta.(type) {
	case *metadata.TaskMetadataV3:
		cluster = m.Cluster
		taskArn = m.TaskARN
	case *metadata.TaskMetadataV4:
		cluster = m.Cluster
		taskArn = m.TaskARN
	}
	if c.ECS.Cluster == "" && cluster != "" {
		log.Printf("[info] ECS cluster is set from task metadata: %s", cluster)
		c.ECS.Cluster = cluster
	}

	svc := ecs.NewFromConfig(*c.awscfg)
	if out, err := svc.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(cluster),
		Tasks:   []string{taskArn},
	}); err != nil {
		return err
	} else {
		if len(out.Tasks) == 0 {
			return fmt.Errorf("cannot find task: %s", taskArn)
		}
		group := aws.ToString(out.Tasks[0].Group)
		if strings.HasPrefix(group, "service:") {
			service = group[8:]
		}
	}
	if out, err := svc.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: []string{service},
	}); err != nil {
		return err
	} else {
		if len(out.Services) == 0 {
			return fmt.Errorf("cannot find service: %s", service)
		}
		if c.ECS.networkConfiguration == nil {
			c.ECS.networkConfiguration = out.Services[0].NetworkConfiguration
			log.Printf("[info] network_configuration is not set, using network_configuration=%v", c.ECS.networkConfiguration)
		}
	}
	return nil
}

func (cfg *Config) AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		isAPIRequest := strings.HasPrefix(c.Request().URL.Path, "/api/")
		methods := []AuthMethod{cfg.Auth.ByToken}
		if !isAPIRequest {
			// web access allows other auth methods
			methods = append(methods,
				cfg.Auth.ByCookie,
				cfg.Auth.ByAmznOIDC,
				cfg.Auth.ByBasic, // basic auth must be evaluated at last
			)
		}
		ok, err := cfg.Auth.Do(c.Request(), c.Response(), methods...)
		if err != nil {
			log.Println("[error] auth error:", err)
			return echo.ErrInternalServerError
		}
		if !ok {
			log.Println("[warn] auth failed")
			return echo.ErrUnauthorized
		}

		// set auth cookie for web access
		if !isAPIRequest {
			cookie, err := cfg.Auth.NewAuthCookie(AuthCookieExpire, cfg.Host.ReverseProxySuffix)
			if err != nil {
				log.Println("[error] failed to create auth cookie:", err)
				return echo.ErrInternalServerError
			}
			if cookie.Value != "" {
				c.SetCookie(cookie)
			}
		}
		return next(c)
	}
}

func loadFromFile(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func loadFromS3(ctx context.Context, awscfg *aws.Config, u string) ([]byte, error) {
	svc := s3.NewFromConfig(*awscfg)
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "s3" {
		return nil, fmt.Errorf("invalid scheme: %s", parsed.Scheme)
	}
	bucket := parsed.Host
	key := strings.TrimPrefix(parsed.Path, "/")
	out, err := svc.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func (c *Config) downloadHTMLFromS3(ctx context.Context) error {
	log.Printf("[info] downloading html files from %s", c.HtmlDir)
	tmpdir, err := os.MkdirTemp("", "mirage-ecs-htmldir-")
	if err != nil {
		return err
	}
	svc := s3.NewFromConfig(*c.awscfg)
	parsed, err := url.Parse(c.HtmlDir)
	if err != nil {
		return err
	}
	if parsed.Scheme != "s3" {
		return fmt.Errorf("invalid scheme: %s", parsed.Scheme)
	}
	bucket := parsed.Host
	keyPrefix := strings.TrimPrefix(parsed.Path, "/")
	if !strings.HasSuffix(keyPrefix, "/") {
		keyPrefix += "/"
	}
	log.Println("[debug] bucket:", bucket, "keyPrefix:", keyPrefix)
	res, err := svc.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(keyPrefix),
		Delimiter: aws.String("/"),
		MaxKeys:   100, // sufficient for html template files
	})
	if err != nil {
		return err
	}
	if len(res.Contents) == 0 {
		return fmt.Errorf("no objects found in %s", c.HtmlDir)
	}
	files := 0
	for _, obj := range res.Contents {
		log.Printf("[info] downloading %s", aws.ToString(obj.Key))
		r, err := svc.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    obj.Key,
		})
		if err != nil {
			return err
		}
		defer r.Body.Close()
		filename := path.Base(aws.ToString(obj.Key))
		file := filepath.Join(tmpdir, filename)
		if size, err := copyToFile(r.Body, file); err != nil {
			return err
		} else {
			files++
			log.Printf("[info] downloaded %s (%d bytes)", file, size)
		}
	}
	log.Printf("[info] downloaded %d files from %s", files, c.HtmlDir)
	c.HtmlDir = tmpdir
	c.cleanups = append(c.cleanups, func() error {
		log.Printf("[info] removing %s", tmpdir)
		return os.RemoveAll(tmpdir)
	})
	log.Printf("[debug] setting html dir: %s", c.HtmlDir)
	return nil
}

func copyToFile(src io.Reader, dst string) (int64, error) {
	f, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, src)
}
