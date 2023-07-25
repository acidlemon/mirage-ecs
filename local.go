package mirageecs

import (
	"context"
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

// LocalTaskRunner is a mock implementation of TaskRunner.
type LocalTaskRunner struct {
	Informations map[string]Information

	stopServerFuncs map[string]func()
	cfg             *Config
	proxyControlCh  chan *proxyControl
}

func NewLocalTaskRunner(cfg *Config) TaskRunner {
	return &LocalTaskRunner{
		Informations:    map[string]Information{},
		stopServerFuncs: map[string]func(){},
		cfg:             cfg,
	}
}

func (e *LocalTaskRunner) SetProxyControlChannel(ch chan *proxyControl) {
	e.proxyControlCh = ch
}

func (e *LocalTaskRunner) List(_ context.Context, status string) ([]Information, error) {
	infos := make([]Information, 0, len(e.Informations))
	for _, info := range e.Informations {
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Created.After(infos[j].Created)
	})
	return infos, nil
}

func (e *LocalTaskRunner) Launch(ctx context.Context, subdomain string, option TaskParameter, taskdefs ...string) error {
	if info, ok := e.Informations[subdomain]; ok {
		return fmt.Errorf("subdomain %s is already used by %s", subdomain, info.ID)
	}
	id := generateRandomHexID(32)
	env := option.ToEnv(subdomain, e.cfg.Parameter)
	log.Printf("[info] Launching a new mock task: subdomain=%s, taskdef=%s, id=%s", subdomain, taskdefs[0], id)
	contents := fmt.Sprintf("Hello, Mirage! subdomain: %s\n%#v", subdomain, env)
	port, stopServerFunc := runMockServer(contents)
	e.Informations[subdomain] = Information{
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
		tags: option.ToECSTags(subdomain, e.cfg.Parameter),
	}
	e.stopServerFuncs[id] = stopServerFunc
	e.proxyControlCh <- &proxyControl{
		Action:    proxyAdd,
		Subdomain: subdomain,
		IPAddress: "127.0.0.1",
		Port:      port,
	}
	return nil
}

func (e *LocalTaskRunner) Logs(subdomain string, since time.Time, tail int) ([]string, error) {
	// Logs returns logs of the specified subdomain.
	return []string{"Sorry. mock server logs are empty."}, nil
}

func (e *LocalTaskRunner) Terminate(id string) error {
	for _, info := range e.Informations {
		if info.ID == id {
			return e.TerminateBySubdomain(info.SubDomain)
		}
	}
	return nil
}

func (e *LocalTaskRunner) TerminateBySubdomain(subdomain string) error {
	log.Printf("[info] Terminating a mock task: subdomain=%s", subdomain)
	if info, ok := e.Informations[subdomain]; ok {
		if stop := e.stopServerFuncs[info.ShortID]; stop != nil {
			stop()
		}
		e.proxyControlCh <- &proxyControl{
			Action:    proxyRemove,
			Subdomain: subdomain,
		}
		delete(e.Informations, subdomain)
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

func (e *LocalTaskRunner) GetAccessCount(_ context.Context, subdomain string, duration time.Duration) (int64, error) {
	log.Println("[debug] GetAccessCount is not implemented in LocalTaskRunner")
	return 0, nil
}

func (e *LocalTaskRunner) PutAccessCounts(_ context.Context, _ map[string]accessCount) error {
	log.Println("[debug] PutAccessCounts is not implemented in LocalTaskRunner")
	return nil
}
