package mirageecs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/google/uuid"
	"github.com/samber/lo"
)

// LocalTaskRunner is a mock implementation of TaskRunner.
type LocalTaskRunner struct {
	Informations []*Information

	stopServerFuncs map[string]func()
	cfg             *Config
	proxyControlCh  chan *proxyControl
}

func NewLocalTaskRunner(cfg *Config) TaskRunner {
	return &LocalTaskRunner{
		Informations:    []*Information{},
		stopServerFuncs: map[string]func(){},
		cfg:             cfg,
	}
}

func (e *LocalTaskRunner) SetProxyControlChannel(ch chan *proxyControl) {
	e.proxyControlCh = ch
}

func (e *LocalTaskRunner) List(_ context.Context, status string) ([]*Information, error) {
	infos := lo.Filter(e.Informations, func(info *Information, _ int) bool {
		return *info.task.DesiredStatus == status
	})
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].task.CreatedAt.After(*infos[j].task.CreatedAt)
	})
	return infos, nil
}

func (e *LocalTaskRunner) Trace(_ context.Context, id string) (string, error) {
	return fmt.Sprintf("mock trace of %s", id), nil
}

func (e *LocalTaskRunner) Launch(ctx context.Context, subdomain string, option TaskParameter, launchType LaunchType, taskdefs ...string) error {
	if infos := e.find(subdomain); 0 < len(infos) {
		slog.Info(f("subdomain %s is already running task id %v. Terminating...", subdomain, infos.ShortIDs()))
		err := e.TerminateBySubdomain(ctx, subdomain)
		if err != nil {
			return err
		}
	}
	_commonID, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	commonID := _commonID.String()
	for _, taskdef := range taskdefs {
		id := generateRandomHexID(32)
		env := option.ToEnv(subdomain, e.cfg.Parameter, e.cfg.EncodeSubdomain, launchType)
		slog.Info(f("Launching a new mock task: subdomain=%s, taskdef=%s, id=%s", subdomain, taskdef, id))
		contents := fmt.Sprintf("Hello, Mirage! subdomain: %s\n%#v", subdomain, env)
		port, stopServerFunc := runMockServer(contents)
		e.Informations = append(e.Informations, &Information{
			ID:         "arn:aws:ecs:ap-northeast-1:123456789012:task/mirage/" + id,
			ShortID:    id,
			CommonID:   commonID,
			SubDomain:  subdomain,
			GitBranch:  option["branch"],
			TaskDef:    taskdef,
			IPAddress:  "127.0.0.1",
			Created:    time.Now().In(time.Local),
			LastStatus: statusRunning,
			PortMap: map[string]int{
				"httpd": port,
			},
			Env:  env,
			Tags: option.ToECSTags(subdomain, e.cfg.Parameter, commonID, launchType),
			task: &types.Task{
				LastStatus:    aws.String(statusRunning),
				DesiredStatus: aws.String(statusRunning),
				CreatedAt:     aws.Time(time.Now().UTC()),
				StartedAt:     aws.Time(time.Now().UTC()),
			},
		})
		e.stopServerFuncs[id] = stopServerFunc
		e.proxyControlCh <- &proxyControl{
			Action:    proxyAdd,
			Subdomain: subdomain,
			IPAddress: "127.0.0.1",
			Port:      port,
		}
	}
	return nil
}

func (e *LocalTaskRunner) Logs(_ context.Context, subdomain string, since time.Time, tail int) ([]string, error) {
	// Logs returns logs of the specified subdomain.
	return []string{"Sorry. mock server logs are empty."}, nil
}

func (e *LocalTaskRunner) Terminate(ctx context.Context, id string) error {
	for _, info := range e.Informations {
		if info.ID == id {
			if stop := e.stopServerFuncs[info.ShortID]; stop != nil {
				stop()
			}
			info.task.LastStatus = aws.String(statusStopped)
			info.task.DesiredStatus = aws.String(statusStopped)
			syncTaskToInfomation(info)
			return nil
		}
	}
	return nil
}

func (e *LocalTaskRunner) find(subdomain string) Informations {
	ret := make(Informations, 0, len(e.Informations))
	for _, info := range e.Informations {
		if info.SubDomain == subdomain && *info.task.DesiredStatus == statusRunning {
			ret = append(ret, info)
		}
	}
	return ret
}

func (e *LocalTaskRunner) TerminateBySubdomain(ctx context.Context, subdomain string) error {
	slog.Info(f("Terminating a mock task: subdomain=%s", subdomain))
	for _, info := range e.find(subdomain) {
		e.Terminate(ctx, info.ID)
		e.proxyControlCh <- &proxyControl{
			Action:    proxyRemove,
			Subdomain: subdomain,
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
	slog.Info(f("mock server is running at %s", ts.URL))
	u, _ := url.Parse(ts.URL)
	port, _ := strconv.Atoi(u.Port())
	return port, ts.Close
}

func (e *LocalTaskRunner) GetAccessCount(_ context.Context, subdomain string, duration time.Duration) (int64, error) {
	slog.Debug("GetAccessCount is not implemented in LocalTaskRunner")
	return 0, nil
}

func (e *LocalTaskRunner) PutAccessCounts(_ context.Context, _ map[string]accessCount) error {
	slog.Debug("PutAccessCounts is not implemented in LocalTaskRunner")
	return nil
}

func syncTaskToInfomation(info *Information) {
	info.LastStatus = *info.task.LastStatus
	info.Created = time.Time{}
	if v := info.task.StartedAt; v != nil {
		info.Created = (*v).In(time.Local)
	}
}
