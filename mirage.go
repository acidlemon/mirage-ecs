package mirageecs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

//var app *Mirage

type Mirage struct {
	Config       *Config
	WebApi       *WebApi
	ReverseProxy *ReverseProxy
	Route53      *Route53
	ECS          ECSInterface

	proxyCh chan *proxyControl
}

func Setup(cfg *Config) *Mirage {
	// launch server
	var e ECSInterface
	if cfg.localMode {
		e = NewECSLocal(cfg)
	} else {
		e = NewECS(cfg)
	}
	proxyCh := make(chan *proxyControl, 10)
	e.SetProxyControlChannel(proxyCh)
	m := &Mirage{
		Config:       cfg,
		ReverseProxy: NewReverseProxy(cfg),
		WebApi:       NewWebApi(cfg, e),
		Route53:      NewRoute53(cfg),
		ECS:          e,
		proxyCh:      proxyCh,
	}
	return m
}

func (m *Mirage) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, v := range m.Config.Listen.HTTP {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			laddr := fmt.Sprintf("%s:%d", m.Config.Listen.ForeignAddress, port)
			listener, err := net.Listen("tcp", laddr)
			if err != nil {
				log.Printf("[error] cannot listen %s: %s", laddr, err)
				return
			}

			mux := http.NewServeMux()
			mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
				m.ServeHTTPWithPort(w, req, port)
			})

			log.Println("[info] listen port:", port)
			http.Serve(listener, mux)
		}(v.ListenPort)
	}

	wg.Add(2)
	go m.syncECSToMirage(ctx, &wg)
	go m.RunAccessCountCollector(ctx, &wg)
	log.Println("[info] Launch succeeded!")

	wg.Wait()
}

func (m *Mirage) ServeHTTPWithPort(w http.ResponseWriter, req *http.Request, port int) {
	host := strings.ToLower(strings.Split(req.Host, ":")[0])

	switch {
	case m.isWebApiHost(host):
		m.WebApi.ServeHTTP(w, req)

	case m.isDockerHost(host):
		m.ReverseProxy.ServeHTTPWithPort(w, req, port)

	case strings.HasSuffix(host, m.Config.Host.ReverseProxySuffix):
		msg := fmt.Sprintf("%s is not found", host)
		log.Println("[warn]", msg)
		http.Error(w, msg, http.StatusNotFound)

	default:
		// not a vhost, returns 200 (for healthcheck)
		http.Error(w, "mirage-ecs", http.StatusOK)
	}

}

func (m *Mirage) isDockerHost(host string) bool {
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
			log.Println("[debug] RunAccessCountCollector() is done")
			return
		}
		all := m.ReverseProxy.CollectAccessCounts()
		s, _ := json.Marshal(all)
		log.Printf("[info] access counters: %s", string(s))
		m.ECS.PutAccessCounts(all)
	}
}

const (
	CloudWatchMetricNameSpace = "mirage-ecs"
	CloudWatchMetricName      = "RequestCount"
	CloudWatchDimensionName   = "subdomain"
)

func (app *Mirage) syncECSToMirage(ctx context.Context, wg *sync.WaitGroup) {
	wg.Done()
	log.Println("[debug] starting up syncECSToMirage()")
	rp := app.ReverseProxy
	r53 := app.Route53
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

SYNC:
	for {
		select {
		case msg := <-app.proxyCh:
			log.Printf("[debug] proxyControl %#v", msg)
			rp.Modify(msg)
			continue SYNC
		case <-ticker.C:
		case <-ctx.Done():
			log.Println("[debug] syncECSToMirage() is done")
			return
		}

		running, err := app.ECS.List(statusRunning)
		if err != nil {
			log.Println("[warn]", err)
			continue
		}
		sort.SliceStable(running, func(i, j int) bool {
			return running[i].Created.Before(running[j].Created)
		})
		available := make(map[string]bool)
		for _, info := range running {
			log.Println("[debug] ruuning task", info.ID)
			if info.IPAddress != "" {
				available[info.SubDomain] = true
				for name, port := range info.PortMap {
					rp.AddSubdomain(info.SubDomain, info.IPAddress, port)
					r53.Add(name+"."+info.SubDomain, info.IPAddress)
				}
			}
		}

		stopped, err := app.ECS.List(statusStopped)
		if err != nil {
			log.Println("[warn]", err)
			continue
		}
		for _, info := range stopped {
			log.Println("[debug] stopped task", info.ID)
			for name := range info.PortMap {
				r53.Delete(name+"."+info.SubDomain, info.IPAddress)
			}
		}

		for _, subdomain := range rp.Subdomains() {
			if !available[subdomain] {
				rp.RemoveSubdomain(subdomain)
			}
		}
		if err := r53.Apply(); err != nil {
			log.Println("[warn]", err)
		}
	}
}
