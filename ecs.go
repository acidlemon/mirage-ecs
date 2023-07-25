package mirageecs

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	ttlcache "github.com/ReneKroon/ttlcache/v2"

	"github.com/aws/aws-sdk-go-v2/aws"
	cw "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cwlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"golang.org/x/sync/errgroup"
)

var taskDefinitionCache = ttlcache.NewCache() // no need to expire because taskdef is immutable.

type Information struct {
	ID         string            `json:"id"`
	ShortID    string            `json:"short_id"`
	SubDomain  string            `json:"subdomain"`
	GitBranch  string            `json:"branch"`
	TaskDef    string            `json:"taskdef"`
	IPAddress  string            `json:"ipaddress"`
	Created    time.Time         `json:"created"`
	LastStatus string            `json:"last_status"`
	PortMap    map[string]int    `json:"port_map"`
	Env        map[string]string `json:"env"`

	tags []types.Tag
	task *types.Task
}

type TaskParameter map[string]string

func (p TaskParameter) ToECSKeyValuePairs(subdomain string, configParams Parameters) []types.KeyValuePair {
	kvp := make([]types.KeyValuePair, 0, len(p)+1)
	kvp = append(kvp, types.KeyValuePair{
		Name:  aws.String(strings.ToUpper(TagSubdomain)),
		Value: aws.String(encodeTagValue(subdomain)),
	})
	for _, v := range configParams {
		v := v
		if p[v.Name] == "" {
			continue
		}
		kvp = append(kvp, types.KeyValuePair{
			Name:  aws.String(v.Env),
			Value: aws.String(p[v.Name]),
		})
	}
	return kvp
}

func (p TaskParameter) ToECSTags(subdomain string, configParams Parameters) []types.Tag {
	tags := make([]types.Tag, 0, len(p)+2)
	tags = append(tags,
		types.Tag{
			Key:   aws.String(TagSubdomain),
			Value: aws.String(encodeTagValue(subdomain)),
		},
		types.Tag{
			Key:   aws.String(TagManagedBy),
			Value: aws.String(TagValueMirage),
		},
	)
	for _, v := range configParams {
		v := v
		if p[v.Name] == "" {
			continue
		}
		tags = append(tags, types.Tag{
			Key:   aws.String(v.Name),
			Value: aws.String(p[v.Name]),
		})
	}
	return tags
}

func (p TaskParameter) ToEnv(subdomain string, configParams Parameters) map[string]string {
	env := make(map[string]string, len(p)+1)
	env[strings.ToUpper(TagSubdomain)] = encodeTagValue(subdomain)
	for _, v := range configParams {
		v := v
		if p[v.Name] == "" {
			continue
		}
		env[strings.ToUpper(v.Env)] = p[v.Name]
	}
	return env
}

const (
	TagManagedBy   = "ManagedBy"
	TagSubdomain   = "Subdomain"
	TagValueMirage = "Mirage"

	statusRunning = "RUNNING"
	statusStopped = "STOPPED"
)

type TaskRunner interface {
	Launch(ctx context.Context, subdomain string, param TaskParameter, taskdefs ...string) error
	Logs(ctx context.Context, subdomain string, since time.Time, tail int) ([]string, error)
	Terminate(ctx context.Context, subdomain string) error
	TerminateBySubdomain(ctx context.Context, subdomain string) error
	List(ctx context.Context, status string) ([]Information, error)
	SetProxyControlChannel(ch chan *proxyControl)
	GetAccessCount(ctx context.Context, subdomain string, duration time.Duration) (int64, error)
	PutAccessCounts(context.Context, map[string]accessCount) error
}

type ECS struct {
	cfg            *Config
	svc            *ecs.Client
	logsSvc        *cwlogs.Client
	cwSvc          *cw.Client
	proxyControlCh chan *proxyControl
}

func NewECSTaskRunner(cfg *Config) TaskRunner {
	e := &ECS{
		cfg:     cfg,
		logsSvc: cwlogs.NewFromConfig(*cfg.awscfg),
		cwSvc:   cw.NewFromConfig(*cfg.awscfg),
	}
	return e
}

func (e *ECS) SetProxyControlChannel(ch chan *proxyControl) {
	e.proxyControlCh = ch
}

func (e *ECS) launchTask(ctx context.Context, subdomain string, taskdef string, option TaskParameter) error {
	cfg := e.cfg

	log.Printf("[info] launching task subdomain:%s taskdef:%s", subdomain, taskdef)
	tdOut, err := e.svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskdef),
	})
	if err != nil {
		return fmt.Errorf("failed to describe task definition: %w", err)
	}

	// override envs for each container in taskdef
	ov := &types.TaskOverride{}
	env := option.ToECSKeyValuePairs(subdomain, cfg.Parameter)

	for _, c := range tdOut.TaskDefinition.ContainerDefinitions {
		name := *c.Name
		ov.ContainerOverrides = append(
			ov.ContainerOverrides,
			types.ContainerOverride{
				Name:        aws.String(name),
				Environment: env,
			},
		)
	}
	log.Printf("[debug] Task Override: %v", ov)

	tags := option.ToECSTags(subdomain, cfg.Parameter)
	runtaskInput := &ecs.RunTaskInput{
		CapacityProviderStrategy: cfg.ECS.capacityProviderStrategy,
		Cluster:                  aws.String(cfg.ECS.Cluster),
		TaskDefinition:           aws.String(taskdef),
		NetworkConfiguration:     cfg.ECS.networkConfiguration,
		LaunchType:               types.LaunchType(*cfg.ECS.LaunchType),
		Overrides:                ov,
		Count:                    aws.Int32(1),
		Tags:                     tags,
		EnableExecuteCommand:     aws.ToBool(cfg.ECS.EnableExecuteCommand),
	}
	log.Printf("[debug] RunTaskInput: %v", runtaskInput)
	out, err := e.svc.RunTask(ctx, runtaskInput)
	if err != nil {
		return err
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		return fmt.Errorf(
			"run task failed. reason:%s arn:%s", *f.Reason, *f.Arn,
		)
	}
	task := out.Tasks[0]
	log.Printf("[info] launced task ARN: %s", *task.TaskArn)
	return nil
}

func (e *ECS) Launch(ctx context.Context, subdomain string, option TaskParameter, taskdefs ...string) error {
	if infos, err := e.find(ctx, subdomain); err != nil {
		return fmt.Errorf("failed to get subdomain %s: %w", subdomain, err)
	} else if len(infos) > 0 {
		log.Printf("[info] subdomain %s is already running %d tasks. Terminating...", subdomain, len(infos))
		err := e.TerminateBySubdomain(ctx, subdomain)
		if err != nil {
			return err
		}
	}

	log.Printf("[info] launching subdomain:%s taskdefs:%v", subdomain, taskdefs)

	var eg errgroup.Group
	for _, taskdef := range taskdefs {
		taskdef := taskdef
		eg.Go(func() error {
			return e.launchTask(ctx, subdomain, taskdef, option)
		})
	}
	return eg.Wait()
}

func (e *ECS) Logs(ctx context.Context, subdomain string, since time.Time, tail int) ([]string, error) {
	infos, err := e.find(ctx, subdomain)
	if err != nil {
		return nil, err
	}
	if len(infos) == 0 {
		return nil, fmt.Errorf("subdomain %s is not found", subdomain)
	}

	var logs []string
	var eg errgroup.Group
	var mu sync.Mutex
	for _, info := range infos {
		info := info
		eg.Go(func() error {
			l, err := e.logs(ctx, info, since, tail)
			mu.Lock()
			defer mu.Unlock()
			logs = append(logs, l...)
			return err
		})
	}
	return logs, eg.Wait()
}

func (e *ECS) logs(ctx context.Context, info Information, since time.Time, tail int) ([]string, error) {
	task := info.task
	taskdefOut, err := e.svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: task.TaskDefinitionArn,
		Include:        []types.TaskDefinitionField{types.TaskDefinitionFieldTags},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe task definition: %w", err)
	}

	streams := make(map[string][]string)
	for _, c := range taskdefOut.TaskDefinition.ContainerDefinitions {
		c := c
		logConf := c.LogConfiguration
		if logConf.LogDriver != types.LogDriverAwslogs {
			log.Printf("[warn] LogDriver %s is not supported", logConf.LogDriver)
			continue
		}
		group := logConf.Options["awslogs-group"]
		streamPrefix := logConf.Options["awslogs-stream-prefix"]
		if group == "" || streamPrefix == "" {
			log.Printf("[warn] invalid options. awslogs-group %s awslogs-stream-prefix %s", group, streamPrefix)
			continue
		}
		// streamName: prefix/containerName/taskID
		streams[group] = append(
			streams[group],
			fmt.Sprintf("%s/%s/%s", streamPrefix, *c.Name, info.ShortID),
		)
	}

	logs := []string{}
	for group, streamNames := range streams {
		group := group
		for _, stream := range streamNames {
			stream := stream
			log.Printf("[debug] get log events from group:%s stream:%s start:%s", group, stream, since)
			in := &cwlogs.GetLogEventsInput{
				LogGroupName:  aws.String(group),
				LogStreamName: aws.String(stream),
			}
			if !since.IsZero() {
				in.StartTime = aws.Int64(since.Unix() * 1000)
			}
			eventsOut, err := e.logsSvc.GetLogEvents(ctx, in)
			if err != nil {
				log.Printf("[warn] failed to get log events from group %s stream %s: %s", group, stream, err)
				continue
			}
			log.Printf("[debug] %d log events", len(eventsOut.Events))
			for _, ev := range eventsOut.Events {
				logs = append(logs, *ev.Message)
			}
		}
	}
	if tail > 0 && len(logs) >= tail {
		return logs[len(logs)-tail:], nil
	}
	return logs, nil
}

func (e *ECS) Terminate(ctx context.Context, taskArn string) error {
	log.Printf("[info] stop task %s", taskArn)
	_, err := e.svc.StopTask(ctx, &ecs.StopTaskInput{
		Cluster: aws.String(e.cfg.ECS.Cluster),
		Task:    aws.String(taskArn),
		Reason:  aws.String("Terminate requested by Mirage"),
	})
	return err
}

func (e *ECS) TerminateBySubdomain(ctx context.Context, subdomain string) error {
	infos, err := e.find(ctx, subdomain)
	if err != nil {
		return err
	}
	var eg errgroup.Group
	eg.Go(func() error {
		e.proxyControlCh <- &proxyControl{
			Action:    proxyRemove,
			Subdomain: subdomain,
		}
		return nil
	})
	for _, info := range infos {
		info := info
		eg.Go(func() error {
			return e.Terminate(ctx, info.ID)
		})
	}
	return eg.Wait()
}

func (e *ECS) find(ctx context.Context, subdomain string) ([]Information, error) {
	var results []Information

	infos, err := e.List(ctx, statusRunning)
	if err != nil {
		return results, err
	}
	for _, info := range infos {
		if info.SubDomain == subdomain {
			results = append(results, info)
		}
	}
	return results, nil
}

func (e *ECS) List(ctx context.Context, desiredStatus string) ([]Information, error) {
	log.Printf("[debug] call ecs.List(%s)", desiredStatus)
	infos := []Information{}
	var nextToken *string
	cluster := aws.String(e.cfg.ECS.Cluster)
	include := []types.TaskField{types.TaskFieldTags}
	for {
		listOut, err := e.svc.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster:       cluster,
			NextToken:     nextToken,
			DesiredStatus: types.DesiredStatus(desiredStatus),
		})
		if err != nil {
			return infos, fmt.Errorf("failed to list tasks: %w", err)
		}
		if len(listOut.TaskArns) == 0 {
			return infos, nil
		}

		tasksOut, err := e.svc.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: cluster,
			Tasks:   listOut.TaskArns,
			Include: include,
		})
		if err != nil {
			return infos, fmt.Errorf("failed to describe tasks: %w", err)
		}

		for _, task := range tasksOut.Tasks {
			task := task
			if getTagsFromTask(&task, TagManagedBy) != TagValueMirage {
				// task is not managed by Mirage
				continue
			}
			info := Information{
				ID:         *task.TaskArn,
				ShortID:    shortenArn(*task.TaskArn),
				SubDomain:  decodeTagValue(getTagsFromTask(&task, "Subdomain")),
				GitBranch:  getEnvironmentFromTask(&task, "GIT_BRANCH"),
				TaskDef:    shortenArn(*task.TaskDefinitionArn),
				IPAddress:  getIPV4AddressFromTask(&task),
				LastStatus: *task.LastStatus,
				Env:        getEnvironmentsFromTask(&task),
				tags:       task.Tags,
				task:       &task,
			}
			if portMap, err := e.portMapInTask(ctx, &task); err != nil {
				log.Printf("[warn] failed to get portMap in task %s %s", *task.TaskArn, err)
			} else {
				info.PortMap = portMap
			}
			if task.StartedAt != nil {
				info.Created = (*task.StartedAt).In(time.Local)
			}
			infos = append(infos, info)
		}

		nextToken = listOut.NextToken
		if nextToken == nil {
			break
		}
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].SubDomain < infos[j].SubDomain
	})

	return infos, nil
}

func shortenArn(arn string) string {
	p := strings.SplitN(arn, ":", 6)
	if len(p) != 6 {
		return ""
	}
	ps := strings.Split(p[5], "/")
	if len(ps) == 0 {
		return ""
	}
	return ps[len(ps)-1]
}

func getIPV4AddressFromTask(task *types.Task) string {
	if len(task.Attachments) == 0 {
		return ""
	}
	for _, d := range task.Attachments[0].Details {
		if *d.Name == "privateIPv4Address" {
			return *d.Value
		}
	}
	return ""
}

func getTagsFromTask(task *types.Task, name string) string {
	for _, t := range task.Tags {
		if *t.Key == name {
			return *t.Value
		}
	}
	return ""
}

func getEnvironmentFromTask(task *types.Task, name string) string {
	if len(task.Overrides.ContainerOverrides) == 0 {
		return ""
	}
	ov := task.Overrides.ContainerOverrides[0]
	for _, env := range ov.Environment {
		if *env.Name == name {
			return *env.Value
		}
	}
	return ""
}

func getEnvironmentsFromTask(task *types.Task) map[string]string {
	env := map[string]string{}
	if len(task.Overrides.ContainerOverrides) == 0 {
		return env
	}
	ov := task.Overrides.ContainerOverrides[0]
	for _, e := range ov.Environment {
		env[*e.Name] = *e.Value
	}
	return env
}

func encodeTagValue(s string) string {
	return base64.URLEncoding.EncodeToString([]byte(s))
}

func decodeTagValue(s string) string {
	d, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		log.Printf("[warn] failed to decode tag value %s %s", s, err)
		return s
	}
	return string(d)
}

func (e *ECS) portMapInTask(ctx context.Context, task *types.Task) (map[string]int, error) {
	portMap := make(map[string]int)
	tdArn := *task.TaskDefinitionArn
	td, err := taskDefinitionCache.Get(tdArn)
	if err != nil && err == ttlcache.ErrNotFound {
		log.Println("[debug] cache miss for", tdArn)
		out, err := e.svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: &tdArn,
		})
		if err != nil {
			return nil, err
		}
		taskDefinitionCache.Set(tdArn, out.TaskDefinition)
		td = out.TaskDefinition
	} else {
		log.Println("[debug] cache hit for", tdArn)
	}
	if _td, ok := td.(*types.TaskDefinition); ok {
		for _, c := range _td.ContainerDefinitions {
			for _, m := range c.PortMappings {
				if m.HostPort == nil {
					continue
				}
				portMap[*c.Name] = int(*m.HostPort)
			}
		}
	} else {
		log.Println("[warn] invalid type", td)
	}
	return portMap, nil
}

func (e *ECS) GetAccessCount(ctx context.Context, subdomain string, duration time.Duration) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	res, err := e.cwSvc.GetMetricData(ctx, &cw.GetMetricDataInput{
		StartTime: aws.Time(time.Now().Add(-duration)),
		EndTime:   aws.Time(time.Now()),
		MetricDataQueries: []cwTypes.MetricDataQuery{
			{
				Id: aws.String("request_count"),
				MetricStat: &cwTypes.MetricStat{
					Metric: &cwTypes.Metric{
						Dimensions: []cwTypes.Dimension{
							{
								Name:  aws.String(CloudWatchDimensionName),
								Value: aws.String(subdomain),
							},
						},
						MetricName: aws.String(CloudWatchMetricName),
						Namespace:  aws.String(CloudWatchMetricNameSpace),
					},
					Period: aws.Int32(int32(duration.Seconds())),
					Stat:   aws.String("Sum"),
				},
			},
		},
	})
	if err != nil {
		return 0, err
	}
	var sum int64
	for _, v := range res.MetricDataResults {
		for _, vv := range v.Values {
			sum += int64(vv)
		}
	}
	return sum, nil
}

func (e *ECS) PutAccessCounts(ctx context.Context, all map[string]accessCount) error {
	pmInput := cw.PutMetricDataInput{
		Namespace: aws.String(CloudWatchMetricNameSpace),
	}
	for subdomain, counters := range all {
		for ts, count := range counters {
			log.Printf("[debug] access for %s %s %d", subdomain, ts.Format(time.RFC3339), count)
			pmInput.MetricData = append(pmInput.MetricData, cwTypes.MetricDatum{
				MetricName: aws.String(CloudWatchMetricName),
				Timestamp:  aws.Time(ts),
				Value:      aws.Float64(float64(count)),
				Dimensions: []cwTypes.Dimension{
					{
						Name:  aws.String(CloudWatchDimensionName),
						Value: aws.String(subdomain),
					},
				},
			})
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if len(pmInput.MetricData) > 0 {
		_, err := e.cwSvc.PutMetricData(ctx, &pmInput)
		return err
	}
	return nil
}
