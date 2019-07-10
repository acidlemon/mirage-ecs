package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/fsouza/go-dockerclient"
)

type Docker struct {
	cfg     *Config
	Storage *MirageStorage
	Client  *docker.Client
}

func NewDocker(cfg *Config, ms *MirageStorage) *Docker {
	client, err := docker.NewClient(cfg.Docker.Endpoint)
	if err != nil {
		fmt.Println("cannot create docker client")
		return nil
	}
	d := &Docker{
		cfg:     cfg,
		Storage: ms,
		Client:  client,
	}

	return d
}

func (d *Docker) Launch(subdomain string, image string, name string, option map[string]string) error {
	var dockerEnv []string = make([]string, 0)
	for _, v := range d.cfg.Parameter {
		if option[v.Name] == "" {
			continue
		}

		dockerEnv = append(dockerEnv, fmt.Sprintf("%s=%s", v.Env, option[v.Name]))
	}
	dockerEnv = append(dockerEnv, fmt.Sprintf("SUBDOMAIN=%s", subdomain))

	opt := docker.CreateContainerOptions{
		Name: name,
		Config: &docker.Config{
			Image: image,
			Env:   dockerEnv,
		},
		HostConfig: d.cfg.Docker.HostConfig,
	}

	// fill opt.Config.ExposedPorts
	if opt.HostConfig != nil && len(opt.HostConfig.PortBindings) != 0 {
		opt.Config.ExposedPorts = make(map[docker.Port]struct{},
			len(opt.HostConfig.PortBindings))
		for key := range opt.HostConfig.PortBindings {
			opt.Config.ExposedPorts[key] = struct{}{}
		}
	}

	container, err := d.Client.CreateContainer(opt)
	if err != nil {
		fmt.Println("cannot create container")
		return err
	}

	err = d.Client.StartContainer(container.ID, nil)
	if err != nil {
		fmt.Println("cannot start container")
		return err
	}

	container, err = d.Client.InspectContainer(container.ID)

	ms := d.Storage

	// terminate old container
	oldContainerID := d.getContainerIDFromSubdomain(subdomain, ms)
	if oldContainerID != "" {
		err = d.Client.StopContainer(oldContainerID, 5)
		if err != nil {
			fmt.Printf(err.Error()) // TODO log warning
		}
	}

	info := Information{
		ID:        container.ID,
		ShortID:   container.ID[0:12],
		SubDomain: subdomain,
		GitBranch: option["branch"],
		Image:     image,
		IPAddress: container.NetworkSettings.IPAddress,
		Created:   container.Created,
	}
	var infoData []byte
	infoData, err = json.Marshal(info)

	err = ms.Set(fmt.Sprintf("subdomain:%s", subdomain), infoData)
	if err != nil {
		fmt.Println(err)
		return err
	}

	ms.AddToSubdomainMap(subdomain)

	return nil
}

func (d *Docker) getContainerIDFromSubdomain(subdomain string, ms *MirageStorage) string {
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

func (d *Docker) List() ([]Information, error) {
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

	return result, nil
}
