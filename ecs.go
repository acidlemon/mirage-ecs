package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/pkg/errors"
)

type Information struct {
	ID         string    `json:"id"`
	ShortID    string    `json:"short_id"`
	SubDomain  string    `json:"subdomain"`
	GitBranch  string    `json:"branch"`
	Image      string    `json:"image"`
	IPAddress  string    `json:"ipaddress"`
	Created    time.Time `json:"created"`
	LastStatus string    `json:"last_status"`
	task       *ecs.Task `json:"-"`
}

const (
	TagManagedBy   = "ManagedBy"
	TagSubdomain   = "Subdomain"
	TagValueMirage = "Mirage"
)

type NotFoundError string

func (e NotFoundError) Error() string {
	return string(e)
}

func NewNotFoundError(name string) error {
	return NotFoundError(fmt.Sprintf("%s is not found", name))
}

type ECS struct {
	cfg            *Config
	ECS            *ecs.ECS
	CloudWatchLogs *cloudwatchlogs.CloudWatchLogs
}

func NewECS(cfg *Config) *ECS {
	sess := session.Must(session.NewSession(
		&aws.Config{Region: aws.String(cfg.ECS.Region)},
	))

	ecs := &ECS{
		cfg:            cfg,
		ECS:            ecs.New(sess),
		CloudWatchLogs: cloudwatchlogs.New(sess),
	}
	go ecs.updateReverseProxy()
	return ecs
}

func (d *ECS) updateReverseProxy() {
	log.Println("[debug] starting up ECS.updateReverseProxy()")
	rp := app.ReverseProxy
	for {
		infos, err := d.List()
		if err != nil {
			log.Println("[warn]", err)
			time.Sleep(10 * time.Second)
			continue
		}
		available := make(map[string]bool)
		for _, info := range infos {
			available[info.SubDomain] = true
			if !rp.Exists(info.SubDomain) && info.LastStatus == "RUNNING" {
				rp.AddSubdomain(info.SubDomain, info.IPAddress)
			}
		}
		for _, subdomain := range rp.Subdomains() {
			if !available[subdomain] {
				rp.RemoveSubdomain(subdomain)
			}
		}
		time.Sleep(10 * time.Second)
	}
}

func (d *ECS) Launch(subdomain string, taskdef string, option map[string]string) error {
	var dockerEnv []*ecs.KeyValuePair

	for _, v := range d.cfg.Parameter {
		v := v
		if option[v.Name] == "" {
			continue
		}
		dockerEnv = append(dockerEnv, &ecs.KeyValuePair{
			Name:  aws.String(v.Env),
			Value: aws.String(option[v.Name]),
		})
	}
	dockerEnv = append(dockerEnv, &ecs.KeyValuePair{
		Name:  aws.String("SUBDOMAIN"),
		Value: aws.String(encodeTagValue(subdomain)),
	})

	tdOut, err := d.ECS.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskdef),
	})
	if err != nil {
		return errors.Wrap(err, "failed to describe task definition")
	}

	// override envs for each container in taskdef
	ov := &ecs.TaskOverride{}
	for _, c := range tdOut.TaskDefinition.ContainerDefinitions {
		name := *c.Name
		ov.ContainerOverrides = append(
			ov.ContainerOverrides,
			&ecs.ContainerOverride{
				Name:        aws.String(name),
				Environment: dockerEnv,
			},
		)
	}
	log.Printf("[debug] Task Override: %s", ov)

	if info, err := d.Find(subdomain); err != nil {
		switch err.(type) {
		case NotFoundError:
			// do nothing
		default:
			return err
		}
	} else {
		log.Printf("[info] subdomain %s is already running on task %s. Terminate.", subdomain, info.ID)
		err := d.Terminate(info.ID)
		if err != nil {
			return err
		}
	}

	awsvpcCfg := d.cfg.ECS.NetworkConfiguration.AwsVpcConfiguration
	out, err := d.ECS.RunTask(
		&ecs.RunTaskInput{
			Cluster:        aws.String(d.cfg.ECS.Cluster),
			TaskDefinition: aws.String(taskdef),
			NetworkConfiguration: &ecs.NetworkConfiguration{
				AwsvpcConfiguration: &ecs.AwsVpcConfiguration{
					AssignPublicIp: awsvpcCfg.AssignPublicIp,
					Subnets:        awsvpcCfg.Subnets,
					SecurityGroups: awsvpcCfg.SecurityGroups,
				},
			},
			LaunchType: aws.String(d.cfg.ECS.LaunchType),
			Overrides:  ov,
			Count:      aws.Int64(1),
			Tags: []*ecs.Tag{
				&ecs.Tag{Key: aws.String(TagSubdomain), Value: aws.String(encodeTagValue(subdomain))},
				&ecs.Tag{Key: aws.String(TagManagedBy), Value: aws.String(TagValueMirage)},
			},
		},
	)
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

func (d *ECS) Logs(subdomain string, since time.Time, tail int) ([]string, error) {
	info, err := d.Find(subdomain)
	if err != nil {
		return nil, fmt.Errorf("subdomain %s is not found", subdomain)
	}
	task := info.task
	if task == nil {
		return nil, fmt.Errorf("no task for subdomain %s", subdomain)
	}

	taskdefOut, err := d.ECS.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
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
			eventsOut, err := d.CloudWatchLogs.GetLogEvents(in)
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

func (d *ECS) Terminate(taskArn string) error {
	log.Printf("[info] stop task %s", taskArn)
	_, err := d.ECS.StopTask(&ecs.StopTaskInput{
		Cluster: aws.String(d.cfg.ECS.Cluster),
		Task:    aws.String(taskArn),
		Reason:  aws.String("Terminate requested by Mirage"),
	})
	return err
}

func (d *ECS) TerminateBySubdomain(subdomain string) error {
	info, err := d.Find(subdomain)
	if err != nil {
		return err
	}
	return d.Terminate(info.ID)
}

func (d *ECS) Find(subdomain string) (Information, error) {
	infos, err := d.List()
	if err != nil {
		return Information{}, err
	}
	for _, info := range infos {
		if info.SubDomain == subdomain {
			return info, nil
		}
	}
	return Information{}, NewNotFoundError(subdomain)
}

func (d *ECS) List() ([]Information, error) {
	infos := []Information{}
	var nextToken *string
	cluster := aws.String(d.cfg.ECS.Cluster)
	include := []*string{aws.String("TAGS")}
	for {
		listOut, err := d.ECS.ListTasks(&ecs.ListTasksInput{
			Cluster:   cluster,
			NextToken: nextToken,
		})
		if err != nil {
			return infos, errors.Wrap(err, "failed to list tasks")
		}
		if len(listOut.TaskArns) == 0 {
			return infos, nil
		}

		tasksOut, err := d.ECS.DescribeTasks(&ecs.DescribeTasksInput{
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
				Image:      shortenArn(*task.TaskDefinitionArn),
				IPAddress:  getIPV4AddressFromTask(task),
				LastStatus: *task.LastStatus,
				task:       task,
			}
			if task.StartedAt != nil {
				info.Created = *task.StartedAt
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

func encodeTagValue(s string) string {
	return base64.URLEncoding.EncodeToString([]byte(s))
}

func decodeTagValue(s string) string {
	d, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		log.Println("[warn] failed to decode tag value %s %s", s, err)
		return s
	}
	return string(d)
}
