package main

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/pkg/errors"
)

const (
	TagManagedBy   = "ManagedBy"
	TagName        = "Name"
	TagSubdomain   = "Subdomain"
	TagValueMirage = "Mirage"
)

type ECS struct {
	cfg     *Config
	Storage *MirageStorage
	Client  *ecs.ECS
}

func NewECS(cfg *Config, ms *MirageStorage) *ECS {
	sess := session.Must(session.NewSession(
		&aws.Config{Region: aws.String(cfg.ECS.Region)},
	))

	return &ECS{
		cfg:     cfg,
		Storage: ms,
		Client:  ecs.New(sess),
	}
}

func (d *ECS) Launch(subdomain string, taskdef string, name string, option map[string]string) error {
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
		Value: aws.String(subdomain),
	})

	tdOut, err := d.Client.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
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
	log.Printf("Task Override: %s", ov)

	awsvpcCfg := d.cfg.ECS.NetworkConfiguration.AwsVpcConfiguration
	out, err := d.Client.RunTask(
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
				&ecs.Tag{Key: aws.String(TagName), Value: aws.String(name)},
				&ecs.Tag{Key: aws.String(TagSubdomain), Value: aws.String(subdomain)},
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
	log.Printf("Task ARN: %s", *task.TaskArn)

	return nil
}

/*
func (d *Docker) Logs(subdomain, since, tail string) ([]string, error) {
	buf := &bytes.Buffer{}

	var parsedSince int64
	if since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err != nil {
			return nil, fmt.Errorf("cannot parse since: %s", err)
		}
		parsedSince = t.Unix()
	}
	containerID := d.getContainerIDFromSubdomain(subdomain, d.Storage)
	if containerID == "" {
		return nil, fmt.Errorf("subdomain=%s is not found", subdomain)
	}

	opt := docker.LogsOptions{
		Container:    containerID,
		OutputStream: buf,
		ErrorStream:  buf,
		Tail:         tail,

		Since:  parsedSince,
		Stdout: true,
		Stderr: true,
	}

	err := d.Client.Logs(opt)
	if err != nil {
		return nil, fmt.Errorf("fail to output logs %s", err)
	}

	scanner := bufio.NewScanner(buf)

	logs := make([]string, 0, 50)
	for scanner.Scan() {
		logs = append(logs, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("fail to scan outputs of log: %s", err)
	}

	return logs, nil
}
*/

func (d *ECS) Terminate(taskArn string) error {
	_, err := d.Client.StopTask(&ecs.StopTaskInput{
		Cluster: aws.String(d.cfg.ECS.Cluster),
		Task:    aws.String(taskArn),
		Reason:  aws.String("Terminate requested by Mirage"),
	})
	return err
}

func (d *ECS) List() ([]Information, error) {
	infos := []Information{}
	var nextToken *string
	cluster := aws.String(d.cfg.ECS.Cluster)
	include := []*string{aws.String("TAGS")}
	for {
		listOut, err := d.Client.ListTasks(&ecs.ListTasksInput{
			Cluster:   cluster,
			NextToken: nextToken,
		})
		if err != nil {
			return infos, errors.Wrap(err, "failed to list tasks")
		}
		if len(listOut.TaskArns) == 0 {
			return infos, nil
		}

		tasksOut, err := d.Client.DescribeTasks(&ecs.DescribeTasksInput{
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
				ID:        *task.TaskArn,
				ShortID:   shortenArn(*task.TaskArn),
				SubDomain: getTagsFromTask(task, "Subdomain"),
				GitBranch: getEnvironmentFromTask(task, "GIT_BRANCH"),
				Image:     shortenArn(*task.TaskDefinitionArn),
				IPAddress: getIPV4AddressFromTask(task),
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
