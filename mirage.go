package mirageecs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

var Version = "current"

type Mirage struct {
	Config       *Config
	WebApi       *WebApi
	ReverseProxy *ReverseProxy
	Route53      *Route53

	runner         TaskRunner
	proxyControlCh chan *proxyControl
}

func New(ctx context.Context, cfg *Config) *Mirage {
	// launch server
	runner := cfg.NewTaskRunner()
	ch := make(chan *proxyControl, 10)
	runner.SetProxyControlChannel(ch)
	m := &Mirage{
		Config:         cfg,
		ReverseProxy:   NewReverseProxy(cfg),
		WebApi:         NewWebApi(cfg, runner),
		Route53:        NewRoute53(ctx, cfg),
		runner:         runner,
		proxyControlCh: ch,
	}
	return m
}

func (m *Mirage) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errors := make(chan error, 10)
	for _, v := range m.Config.Listen.HTTP {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			laddr := fmt.Sprintf("%s:%d", m.Config.Listen.ForeignAddress, port)
			listener, err := net.Listen("tcp", laddr)
			if err != nil {
				slog.Error(f("cannot listen %s: %s", laddr, err))
				errors <- err
				cancel()
				return
			}

			mux := http.NewServeMux()
			mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
				m.ServeHTTPWithPort(w, req, port)
			})
			slog.Info(f("listen addr: %s", laddr))
			srv := &http.Server{
				Handler: mux,
			}
			go srv.Serve(listener)
			<-ctx.Done()
			slog.Info(f("shutdown server: %s", laddr))
			srv.Shutdown(ctx)
		}(v.ListenPort)
	}

	wg.Add(2)
	go m.syncECSToMirage(ctx, &wg)
	go m.RunAccessCountCollector(ctx, &wg)
	wg.Wait()
	slog.Info("shutdown mirage-ecs")
	select {
	case err := <-errors:
		return err
	default:
	}
	m.Config.Cleanup()
	return nil
}

func (m *Mirage) ServeHTTPWithPort(w http.ResponseWriter, req *http.Request, port int) {
	host := strings.ToLower(strings.Split(req.Host, ":")[0])

	switch {
	case m.isWebApiHost(host):
		m.WebApi.ServeHTTP(w, req)

	case m.isTaskHost(host):
		m.ReverseProxy.ServeHTTPWithPort(w, req, port)

	case strings.HasSuffix(host, m.Config.Host.ReverseProxySuffix):
		msg := fmt.Sprintf("%s is not found", host)
		slog.Warn(msg)
		http.Error(w, msg, http.StatusNotFound)

	default:
		// not a vhost, returns 200 (for healthcheck)
		http.Error(w, "mirage-ecs", http.StatusOK)
	}

}

func (m *Mirage) isTaskHost(host string) bool {
	if strings.HasSuffix(host, m.Config.Host.ReverseProxySuffix) {
		subdomain := strings.ToLower(strings.Split(host, ".")[0])
		return m.ReverseProxy.Exists(subdomain)
	}

	return false
}

func (m *Mirage) isWebApiHost(host string) bool {
	return isSameHost(m.Config.Host.WebApi, host)
}

func isSameHost(s1 string, s2 string) bool {
	lower1 := strings.Trim(strings.ToLower(s1), " ")
	lower2 := strings.Trim(strings.ToLower(s2), " ")

	return lower1 == lower2
}

func (m *Mirage) RunAccessCountCollector(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	tk := time.NewTicker(m.ReverseProxy.accessCounterUnit)
	for {
		select {
		case <-tk.C:
		case <-ctx.Done():
			slog.Warn("RunAccessCountCollector() is done")
			return
		}
		all := m.ReverseProxy.CollectAccessCounts()
		s, _ := json.Marshal(all)
		slog.Info(f("access counters: %s", string(s)))
		m.runner.PutAccessCounts(ctx, all)
	}
}

const (
	CloudWatchMetricNameSpace = "mirage-ecs"
	CloudWatchMetricName      = "RequestCount"
	CloudWatchDimensionName   = "subdomain"
)

func (app *Mirage) syncECSToMirage(ctx context.Context, wg *sync.WaitGroup) {
	wg.Done()
	slog.Debug("starting up syncECSToMirage()")
	rp := app.ReverseProxy
	r53 := app.Route53
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

SYNC:
	for {
		select {
		case msg := <-app.proxyControlCh:
			slog.Debug(f("proxyControl %#v", msg))
			rp.Modify(msg)
			continue SYNC
		case <-ticker.C:
		case <-ctx.Done():
			slog.Debug("syncECSToMirage() is done")
			return
		}

		running, err := app.runner.List(ctx, statusRunning)
		if err != nil {
			slog.Warn(err.Error())
			continue
		}
		sort.SliceStable(running, func(i, j int) bool {
			return running[i].Created.Before(running[j].Created)
		})
		available := make(map[string]bool)
		for _, info := range running {
			slog.Debug(f("ruuning task %s", info.ID))
			if info.IPAddress != "" {
				available[info.SubDomain] = true
				for name, port := range info.PortMap {
					rp.AddSubdomain(info.SubDomain, info.IPAddress, port)
					r53.Add(name+"."+info.SubDomain, info.IPAddress)
				}
			}
		}

		stopped, err := app.runner.List(ctx, statusStopped)
		if err != nil {
			slog.Warn(err.Error())
			continue
		}
		for _, info := range stopped {
			slog.Debug(f("stopped task %s", info.ID))
			for name := range info.PortMap {
				r53.Delete(name+"."+info.SubDomain, info.IPAddress)
			}
		}

		for _, subdomain := range rp.Subdomains() {
			if !available[subdomain] {
				rp.RemoveSubdomain(subdomain)
			}
		}
		if err := r53.Apply(ctx); err != nil {
			slog.Warn(err.Error())
		}
	}
}
