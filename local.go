package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ECSLocal struct {
	// ECSLocal is a local mock ECS implementation for Mirage.
	Informations map[string][]Information

	stopServers map[string]func()
}

func NewECSLocal(cfg *Config) *ECSLocal {
	// NewECSLocal returns a new ECSLocal instance.
	return &ECSLocal{
		Informations: map[string][]Information{},
		stopServers:  map[string]func(){},
	}
}

func (ecs *ECSLocal) Run() {
	// Run starts the ECSLocal instance.
}

func (ecs *ECSLocal) List(status string) ([]Information, error) {
	infos := make([]Information, 0, len(ecs.Informations))
	for _, info := range ecs.Informations {
		for _, i := range info {
			infos = append(infos, i)
		}
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Created.After(infos[j].Created)
	})
	return infos, nil
}

func (ecs *ECSLocal) Launch(subdomain string, option map[string]string, taskdefs ...string) ([]string, error) {
	if infos, ok := ecs.Informations[subdomain]; ok {
		return nil, fmt.Errorf("subdomain %s is already used by %s", subdomain, infos[0].ID)
	}

	var taskArns []string
	for _, taskdef := range taskdefs {
		id := generateRandomHexID(32)
		env := map[string]string{}
		for k, v := range option {
			env[strings.ToUpper(k)] = v
		}
		log.Printf("[info] Launching a new mock task: subdomain=%s, taskdef=%s, id=%s", subdomain, taskdefs[0], id)
		port, stopServer := runMockServer(
			fmt.Sprintf("Hello, Mirage! subdomain:%s taskdef:%s", subdomain, taskdef),
		)
		taskArn := "arn:aws:ecs:ap-northeast-1:123456789012:task/mirage/" + id
		ecs.Informations[subdomain] = append(ecs.Informations[subdomain], Information{
			ID:         taskArn,
			ShortID:    id,
			SubDomain:  subdomain,
			GitBranch:  option["branch"],
			TaskDef:    taskdef,
			IPAddress:  "127.0.0.1",
			Created:    time.Now().UTC(),
			LastStatus: "RUNNING",
			PortMap: map[string]int{
				"httpd": port,
			},
			Env: env,
		})
		ecs.stopServers[id] = stopServer
		app.ReverseProxy.AddSubdomain(subdomain, "127.0.0.1", port)
		taskArns = append(taskArns, taskArn)
	}
	return taskArns, nil
}

func (ecs *ECSLocal) Logs(subdomain string, since time.Time, tail int) ([]string, error) {
	// Logs returns logs of the specified subdomain.
	return []string{"Sorry. mock server logs are empty."}, nil
}

func (ecs *ECSLocal) Terminate(id string) error {
	for _, infos := range ecs.Informations {
		for _, info := range infos {
			if info.ID == id {
				return ecs.TerminateBySubdomain(info.SubDomain)
			}
		}
	}
	return nil
}

func (ecs *ECSLocal) TerminateBySubdomain(subdomain string) error {
	log.Printf("[info] Terminating a mock task: subdomain=%s", subdomain)
	if infos, ok := ecs.Informations[subdomain]; ok {
		for _, info := range infos {
			stopServer := ecs.stopServers[info.ShortID]
			if stopServer != nil {
				stopServer()
			}
			app.ReverseProxy.RemoveSubdomain(info.SubDomain)
			delete(ecs.Informations, subdomain)
		}
	}
	return nil
}

func generateRandomHexID(length int) string {
	idBytes := make([]byte, length/2)
	if _, err := rand.Read(idBytes); err != nil {
		panic(err)
	}
	id := hex.EncodeToString(idBytes)
	return id
}

// run mock http server on ephemeral port at localhost, returns the port number and a function to stop the server
func runMockServer(content string) (int, func()) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, content)
	}))
	log.Println("[info] mock server is running at", ts.URL)
	u, _ := url.Parse(ts.URL)
	port, _ := strconv.Atoi(u.Port())
	return port, ts.Close
}
