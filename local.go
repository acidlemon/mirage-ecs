package mirageecs

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
	"time"
)

type ECSLocal struct {
	// ECSLocal is a local mock ECS implementation for Mirage.
	Informations map[string]Information

	stopServers map[string]func()
	cfg         *Config
}

func NewECSLocal(cfg *Config) *ECSLocal {
	// NewECSLocal returns a new ECSLocal instance.
	return &ECSLocal{
		Informations: map[string]Information{},
		stopServers:  map[string]func(){},
		cfg:          cfg,
	}
}

func (ecs *ECSLocal) Run() {
	// Run starts the ECSLocal instance.
}

func (ecs *ECSLocal) List(status string) ([]Information, error) {
	infos := make([]Information, 0, len(ecs.Informations))
	for _, info := range ecs.Informations {
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Created.After(infos[j].Created)
	})
	return infos, nil
}

func (ecs *ECSLocal) Launch(subdomain string, option TaskParameter, taskdefs ...string) error {
	if info, ok := ecs.Informations[subdomain]; ok {
		return fmt.Errorf("subdomain %s is already used by %s", subdomain, info.ID)
	}
	id := generateRandomHexID(32)
	env := option.ToEnv(subdomain, ecs.cfg.Parameter)
	log.Printf("[info] Launching a new mock task: subdomain=%s, taskdef=%s, id=%s", subdomain, taskdefs[0], id)
	contents := fmt.Sprintf("Hello, Mirage! subdomain: %s\n%#v", subdomain, env)
	port, stopServer := runMockServer(contents)
	ecs.Informations[subdomain] = Information{
		ID:         "arn:aws:ecs:ap-northeast-1:123456789012:task/mirage/" + id,
		ShortID:    id,
		SubDomain:  subdomain,
		GitBranch:  option["branch"],
		TaskDef:    taskdefs[0],
		IPAddress:  "127.0.0.1",
		Created:    time.Now().UTC(),
		LastStatus: "RUNNING",
		PortMap: map[string]int{
			"httpd": port,
		},
		Env:  env,
		tags: option.ToECSTags(subdomain, ecs.cfg.Parameter),
	}
	ecs.stopServers[id] = stopServer
	app.ReverseProxy.AddSubdomain(subdomain, "127.0.0.1", port)
	return nil
}

func (ecs *ECSLocal) Logs(subdomain string, since time.Time, tail int) ([]string, error) {
	// Logs returns logs of the specified subdomain.
	return []string{"Sorry. mock server logs are empty."}, nil
}

func (ecs *ECSLocal) Terminate(id string) error {
	for _, info := range ecs.Informations {
		if info.ID == id {
			return ecs.TerminateBySubdomain(info.SubDomain)
		}
	}
	return nil
}

func (ecs *ECSLocal) TerminateBySubdomain(subdomain string) error {
	log.Printf("[info] Terminating a mock task: subdomain=%s", subdomain)
	if info, ok := ecs.Informations[subdomain]; ok {
		stopServer := ecs.stopServers[info.ShortID]
		if stopServer != nil {
			stopServer()
		}
		app.ReverseProxy.RemoveSubdomain(info.SubDomain)
		delete(ecs.Informations, subdomain)
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
