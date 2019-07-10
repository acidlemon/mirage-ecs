package main

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
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
	var dockerEnv []string = make([]string, 0)
	for _, v := range d.cfg.Parameter {
		if option[v.Name] == "" {
			continue
		}

		dockerEnv = append(dockerEnv, fmt.Sprintf("%s=%s", v.Env, option[v.Name]))
	}
	dockerEnv = append(dockerEnv, fmt.Sprintf("SUBDOMAIN=%s", subdomain))

	//func (d *App) RunTask(ctx context.Context, tdArn string, sv *ecs.Service, ov *ecs.TaskOverride, count int64) (*ecs.Task, error) {
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
			Overrides:  nil,
			Count:      aws.Int64(1),
			Tags: []*ecs.Tag{
				&ecs.Tag{Key: aws.String("Name"), Value: aws.String(name)},
				&ecs.Tag{Key: aws.String("Subdomain"), Value: aws.String(subdomain)},
				&ecs.Tag{Key: aws.String("ManagedBy"), Value: aws.String("Mirage")},
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
func (d *ECS) getContainerIDFromSubdomain(subdomain string, ms *MirageStorage) string {
	data, err := ms.Get(fmt.Sprintf("subdomain:%s", subdomain))
	if err != nil {
		if err == ErrNotFound {
			return ""
		}
		fmt.Printf("cannot find subdomain:%s, err:%s", subdomain, err.Error())
		return ""
	}
	var info Information
	json.Unmarshal(data, &info)
	//dump.Dump(info)
	containerID := string(info.ID)

	return containerID
}

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

func (d *Docker) Terminate(subdomain string) error {
	ms := d.Storage

	containerID := d.getContainerIDFromSubdomain(subdomain, ms)

	err := d.Client.StopContainer(containerID, 5)
	if err != nil {
		return err
	}

	ms.RemoveFromSubdomainMap(subdomain)

	return nil
}

// extends docker.APIContainers for sort pkg
type ContainerSlice []docker.APIContainers

func (c ContainerSlice) Len() int {
	return len(c)
}
func (c ContainerSlice) Less(i, j int) bool {
	return c[i].ID < c[j].ID
}
func (c ContainerSlice) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}
*/
func (d *ECS) List() ([]Information, error) {
	return []Information{}, nil
	/*
		ms := d.Storage
		subdomainList, err := ms.GetSubdomainList()
		if err != nil {
			return nil, err
		}

		containers, _ := d.Client.ListContainers(docker.ListContainersOptions{})
		sort.Sort(ContainerSlice(containers))

		result := []Information{}
		for _, subdomain := range subdomainList {
			infoData, err := ms.Get(fmt.Sprintf("subdomain:%s", subdomain))
			if err != nil {
				fmt.Printf("ms.Get failed err=%s\n", err.Error())
				continue
			}

			var info Information
			err = json.Unmarshal(infoData, &info)
			//dump.Dump(info)

			index := sort.Search(len(containers), func(i int) bool { return containers[i].ID >= info.ID })

			if index < len(containers) && containers[index].ID == info.ID {
				// found
				result = append(result, info)
			}
		}

		return result, nil*/
}
