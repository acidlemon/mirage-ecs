package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	//	"github.com/acidlemon/go-dumper"
	"github.com/methane/rproxy"
)

type ReverseProxy struct {
	mu        sync.RWMutex
	cfg       *Config
	domainMap map[string]ProxyInformation
}

func NewReverseProxy(cfg *Config) *ReverseProxy {
	return &ReverseProxy{
		cfg:       cfg,
		domainMap: map[string]ProxyInformation{},
	}
}

func (r *ReverseProxy) ServeHTTPWithPort(w http.ResponseWriter, req *http.Request, port int) {
	subdomain := strings.ToLower(strings.Split(req.Host, ".")[0])

	if handler := r.findHandler(subdomain, port); handler != nil {
		handler.ServeHTTP(w, req)
	} else {
		http.NotFound(w, req)
	}
}

func (r *ReverseProxy) Exists(subdomain string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.domainMap[subdomain]
	return exists
}

func (r *ReverseProxy) Subdomains() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ds := make([]string, 0, len(r.domainMap))
	for name, _ := range r.domainMap {
		ds = append(ds, name)
	}
	return ds
}

func (r *ReverseProxy) findHandler(subdomain string, port int) http.Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	proxyInfo, ok := r.domainMap[subdomain]
	if !ok {
		return nil
	}

	handler, ok := proxyInfo.proxyHandlers[port]
	if !ok {
		return nil
	}

	return handler
}

type ProxyInformation struct {
	IPAddress     string
	proxyHandlers map[int]http.Handler
}

func (r *ReverseProxy) AddSubdomain(subdomain string, ipaddress string) {
	handlers := make(map[int]http.Handler)

	// create reverse proxy
	for _, v := range r.cfg.Listen.HTTP {
		destUrlString := fmt.Sprintf("http://%s:%d", ipaddress, v.TargetPort)
		destUrl, _ := url.Parse(destUrlString)
		handler := rproxy.NewSingleHostReverseProxy(destUrl)

		handlers[v.ListenPort] = handler
	}

	log.Printf("[info] add subdomain: %s -> %s", subdomain, ipaddress)

	// add to map
	r.mu.Lock()
	defer r.mu.Unlock()
	r.domainMap[subdomain] = ProxyInformation{
		IPAddress:     ipaddress,
		proxyHandlers: handlers,
	}
}

func (r *ReverseProxy) RemoveSubdomain(subdomain string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	log.Println("[info] remove subdomain:", subdomain)
	delete(r.domainMap, subdomain)
}
