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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/pkg/errors"
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

	tags []*ecs.Tag
	task *ecs.Task
}

type TaskParameter map[string]string

func (p TaskParameter) ToECSKeyValuePairs(subdomain string, configParams Parameters) []*ecs.KeyValuePair {
	kvp := make([]*ecs.KeyValuePair, 0, len(p)+1)
	kvp = append(kvp, &ecs.KeyValuePair{
		Name:  aws.String(strings.ToUpper(TagSubdomain)),
		Value: aws.String(encodeTagValue(subdomain)),
	})
	for _, v := range configParams {
		v := v
		if p[v.Name] == "" {
			continue
		}
		kvp = append(kvp, &ecs.KeyValuePair{
			Name:  aws.String(v.Env),
			Value: aws.String(p[v.Name]),
		})
	}
	return kvp
}

func (p TaskParameter) ToECSTags(subdomain string, configParams Parameters) []*ecs.Tag {
	tags := make([]*ecs.Tag, 0, len(p)+2)
	tags = append(tags,
		&ecs.Tag{
			Key:   aws.String(TagSubdomain),
			Value: aws.String(encodeTagValue(subdomain)),
		},
		&ecs.Tag{
			Key:   aws.String(TagManagedBy),
			Value: aws.String(TagValueMirage),
		},
	)
	for _, v := range configParams {
		v := v
		if p[v.Name] == "" {
			continue
		}
		tags = append(tags, &ecs.Tag{
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
	Launch(subdomain string, param TaskParameter, taskdefs ...string) error
	Logs(subdomain string, since time.Time, tail int) ([]string, error)
	Terminate(subdomain string) error
	TerminateBySubdomain(subdomain string) error
	List(status string) ([]Information, error)
	SetProxyControlChannel(ch chan *proxyControl)
	GetAccessCount(subdomain string, duration time.Duration) (int64, error)
	PutAccessCounts(map[string]accessCount) error
}

type ECS struct {
	cfg            *Config
	svc            *ecs.ECS
	logsSvc        *cloudwatchlogs.CloudWatchLogs
	cwSvc          *cloudwatch.CloudWatch
	proxyControlCh chan *proxyControl
}

func NewECSTaskRunner(cfg *Config) TaskRunner {
	e := &ECS{
		cfg:     cfg,
		svc:     ecs.New(cfg.session),
		logsSvc: cloudwatchlogs.New(cfg.session),
		cwSvc:   cloudwatch.New(cfg.session),
	}
	return e
}

func (e *ECS) SetProxyControlChannel(ch chan *proxyControl) {
	e.proxyControlCh = ch
}

func (e *ECS) launchTask(subdomain string, taskdef string, option TaskParameter) error {
	cfg := e.cfg

	log.Printf("[info] launching task subdomain:%s taskdef:%s", subdomain, taskdef)
	tdOut, err := e.svc.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskdef),
	})
	if err != nil {
		return errors.Wrap(err, "failed to describe task definition")
	}

	// override envs for each container in taskdef
	ov := &ecs.TaskOverride{}
	env := option.ToECSKeyValuePairs(subdomain, cfg.Parameter)

	for _, c := range tdOut.TaskDefinition.ContainerDefinitions {
		name := *c.Name
		ov.ContainerOverrides = append(
			ov.ContainerOverrides,
			&ecs.ContainerOverride{
				Name:        aws.String(name),
				Environment: env,
			},
		)
	}
	log.Printf("[debug] Task Override: %s", ov)

	tags := option.ToECSTags(subdomain, cfg.Parameter)
	runtaskInput := &ecs.RunTaskInput{
		CapacityProviderStrategy: cfg.ECS.capacityProviderStrategy,
		Cluster:                  aws.String(cfg.ECS.Cluster),
		TaskDefinition:           aws.String(taskdef),
		NetworkConfiguration:     cfg.ECS.networkConfiguration,
		LaunchType:               cfg.ECS.LaunchType,
		Overrides:                ov,
		Count:                    aws.Int64(1),
		Tags:                     tags,
		EnableExecuteCommand:     cfg.ECS.EnableExecuteCommand,
	}
	log.Printf("[debug] RunTaskInput: %s", runtaskInput)
	out, err := e.svc.RunTask(runtaskInput)
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

func (e *ECS) Launch(subdomain string, option TaskParameter, taskdefs ...string) error {
	if infos, err := e.find(subdomain); err != nil {
		return errors.Wrapf(err, "failed to get subdomain %s", subdomain)
	} else if len(infos) > 0 {
		log.Printf("[info] subdomain %s is already running %d tasks. Terminating...", subdomain, len(infos))
		err := e.TerminateBySubdomain(subdomain)
		if err != nil {
			return err
		}
	}

	log.Printf("[info] launching subdomain:%s taskdefs:%v", subdomain, taskdefs)

	var eg errgroup.Group
	for _, taskdef := range taskdefs {
		taskdef := taskdef
		eg.Go(func() error {
			return e.launchTask(subdomain, taskdef, option)
		})
	}
	return eg.Wait()
}

func (e *ECS) Logs(subdomain string, since time.Time, tail int) ([]string, error) {
	infos, err := e.find(subdomain)
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
			l, err := e.logs(info, since, tail)
			mu.Lock()
			defer mu.Unlock()
			logs = append(logs, l...)
			return err
		})
	}
	return logs, eg.Wait()
}

func (e *ECS) logs(info Information, since time.Time, tail int) ([]string, error) {
	task := info.task
	taskdefOut, err := e.svc.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
		TaskDefinition: task.TaskDefinitionArn,
		Include:        []*string{aws.String("TAGS")},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to describe task definition")
	}

	streams := make(map[string][]string)
	for _, c := range taskdefOut.TaskDefinition.ContainerDefinitions {
		c := c
		logConf := c.LogConfiguration
		if *logConf.LogDriver != "awslogs" {
			log.Printf("[warn] LogDriver %s is not supported", *logConf.LogDriver)
			continue
		}
		group := logConf.Options["awslogs-group"]
		streamPrefix := logConf.Options["awslogs-stream-prefix"]
		if group == nil || streamPrefix == nil {
			log.Printf("[warn] invalid options. awslogs-group %s awslogs-stream-prefix %s", *group, *streamPrefix)
			continue
		}
		// streamName: prefix/containerName/taskID
		streams[*group] = append(
			streams[*group],
			fmt.Sprintf("%s/%s/%s", *streamPrefix, *c.Name, info.ShortID),
		)
	}

	logs := []string{}
	for group, streamNames := range streams {
		group := group
		for _, stream := range streamNames {
			stream := stream
			log.Printf("[debug] get log events from group:%s stream:%s start:%s", group, stream, since)
			in := &cloudwatchlogs.GetLogEventsInput{
				LogGroupName:  aws.String(group),
				LogStreamName: aws.String(stream),
			}
			if !since.IsZero() {
				in.StartTime = aws.Int64(since.Unix() * 1000)
			}
			eventsOut, err := e.logsSvc.GetLogEvents(in)
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

func (e *ECS) Terminate(taskArn string) error {
	log.Printf("[info] stop task %s", taskArn)
	_, err := e.svc.StopTask(&ecs.StopTaskInput{
		Cluster: aws.String(e.cfg.ECS.Cluster),
		Task:    aws.String(taskArn),
		Reason:  aws.String("Terminate requested by Mirage"),
	})
	return err
}

func (e *ECS) TerminateBySubdomain(subdomain string) error {
	infos, err := e.find(subdomain)
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
			return e.Terminate(info.ID)
		})
	}
	return eg.Wait()
}

func (e *ECS) find(subdomain string) ([]Information, error) {
	var results []Information

	infos, err := e.List(statusRunning)
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

func (e *ECS) List(desiredStatus string) ([]Information, error) {
	log.Printf("[debug] call ecs.List(%s)", desiredStatus)
	infos := []Information{}
	var nextToken *string
	cluster := aws.String(e.cfg.ECS.Cluster)
	include := []*string{aws.String("TAGS")}
	for {
		listOut, err := e.svc.ListTasks(&ecs.ListTasksInput{
			Cluster:       cluster,
			NextToken:     nextToken,
			DesiredStatus: &desiredStatus,
		})
		if err != nil {
			return infos, errors.Wrap(err, "failed to list tasks")
		}
		if len(listOut.TaskArns) == 0 {
			return infos, nil
		}

		tasksOut, err := e.svc.DescribeTasks(&ecs.DescribeTasksInput{
			Cluster: cluster,
			Tasks:   listOut.TaskArns,
			Include: include,
		})
		if err != nil {
			return infos, errors.Wrap(err, "failed to describe tasks")
		}

		for _, task := range tasksOut.Tasks {
			task := task
			if getTagsFromTask(task, TagManagedBy) != TagValueMirage {
				// task is not managed by Mirage
				continue
			}
			info := Information{
				ID:         *task.TaskArn,
				ShortID:    shortenArn(*task.TaskArn),
				SubDomain:  decodeTagValue(getTagsFromTask(task, "Subdomain")),
				GitBranch:  getEnvironmentFromTask(task, "GIT_BRANCH"),
				TaskDef:    shortenArn(*task.TaskDefinitionArn),
				IPAddress:  getIPV4AddressFromTask(task),
				LastStatus: *task.LastStatus,
				Env:        getEnvironmentsFromTask(task),
				tags:       task.Tags,
				task:       task,
			}
			if portMap, err := e.portMapInTask(task); err != nil {
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

func getIPV4AddressFromTask(task *ecs.Task) string {
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

func getTagsFromTask(task *ecs.Task, name string) string {
	for _, t := range task.Tags {
		if *t.Key == name {
			return *t.Value
		}
	}
	return ""
}

func getEnvironmentFromTask(task *ecs.Task, name string) string {
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

func getEnvironmentsFromTask(task *ecs.Task) map[string]string {
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

func (e *ECS) portMapInTask(task *ecs.Task) (map[string]int, error) {
	portMap := make(map[string]int)
	tdArn := *task.TaskDefinitionArn
	td, err := taskDefinitionCache.Get(tdArn)
	if err != nil && err == ttlcache.ErrNotFound {
		log.Println("[debug] cache miss for", tdArn)
		out, err := e.svc.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
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
	if _td, ok := td.(*ecs.TaskDefinition); ok {
		for _, c := range _td.ContainerDefinitions {
			for _, m := range c.PortMappings {
				if m.HostPort == nil {
					continue
				}
				portMap[*c.Name] = int(*m.HostPort)
			}
		}
	}
	return portMap, nil
}

func (e *ECS) GetAccessCount(subdomain string, duration time.Duration) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := e.cwSvc.GetMetricDataWithContext(ctx, &cloudwatch.GetMetricDataInput{
		StartTime: aws.Time(time.Now().Add(-duration)),
		EndTime:   aws.Time(time.Now()),
		MetricDataQueries: []*cloudwatch.MetricDataQuery{
			{
				Id: aws.String("request_count"),
				MetricStat: &cloudwatch.MetricStat{
					Metric: &cloudwatch.Metric{
						Dimensions: []*cloudwatch.Dimension{
							{
								Name:  aws.String(CloudWatchDimensionName),
								Value: aws.String(subdomain),
							},
						},
						MetricName: aws.String(CloudWatchMetricName),
						Namespace:  aws.String(CloudWatchMetricNameSpace),
					},
					Period: aws.Int64(int64(duration.Seconds())),
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
			sum += int64(aws.Float64Value(vv))
		}
	}
	return sum, nil
}

func (e *ECS) PutAccessCounts(all map[string]accessCount) error {
	pmInput := cloudwatch.PutMetricDataInput{
		Namespace: aws.String(CloudWatchMetricNameSpace),
	}
	for subdomain, counters := range all {
		for ts, count := range counters {
			log.Printf("[debug] access for %s %s %d", subdomain, ts.Format(time.RFC3339), count)
			pmInput.MetricData = append(pmInput.MetricData, &cloudwatch.MetricDatum{
				MetricName: aws.String(CloudWatchMetricName),
				Timestamp:  aws.Time(ts),
				Value:      aws.Float64(float64(count)),
				Dimensions: []*cloudwatch.Dimension{
					{
						Name:  aws.String(CloudWatchDimensionName),
						Value: aws.String(subdomain),
					},
				},
			})
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if len(pmInput.MetricData) > 0 {
		_, err := e.cwSvc.PutMetricDataWithContext(ctx, &pmInput)
		return err
	}
	return nil
}
