package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
)

var app *Mirage

type Mirage struct {
	Config  *Config
	WebApi  *WebApi
	ECS     *ECS
	Storage *MirageStorage
}

func Setup(cfg *Config) {
	ms := NewMirageStorage(cfg)
	m := &Mirage{
		Config:  cfg,
		WebApi:  NewWebApi(cfg),
		ECS:     NewECS(cfg, ms),
		Storage: ms,
	}

	app = m
}

func Run() {
	// launch server
	var wg sync.WaitGroup
	for _, v := range app.Config.Listen.HTTP {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			laddr := fmt.Sprintf("%s:%d", app.Config.Listen.ForeignAddress, port)
			listener, err := net.Listen("tcp", laddr)
			if err != nil {
				log.Printf("cannot listen %s", laddr)
				return
			}

			mux := http.NewServeMux()
			mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
				app.ServeHTTPWithPort(w, req, port)
			})

			fmt.Println("listen port:", port)
			http.Serve(listener, mux)
		}(v.ListenPort)
	}

	// TODO SSL Support

	wg.Wait()
}

func (m *Mirage) ServeHTTPWithPort(w http.ResponseWriter, req *http.Request, port int) {
	host := strings.ToLower(strings.Split(req.Host, ":")[0])

	switch {
	case m.isWebApiHost(host):
		m.WebApi.ServeHTTP(w, req)

	default:
		// return 404
		http.NotFound(w, req)
	}
}

func (m *Mirage) isWebApiHost(host string) bool {
	return isSameHost(m.Config.Host.WebApi, host)
}

func isSameHost(s1 string, s2 string) bool {
	lower1 := strings.Trim(strings.ToLower(s1), " ")
	lower2 := strings.Trim(strings.ToLower(s2), " ")

	return lower1 == lower2
}
