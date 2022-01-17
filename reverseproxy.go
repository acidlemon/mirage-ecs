package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	//	"github.com/acidlemon/go-dumper"
	"github.com/methane/rproxy"
)

type proxyAction string

const (
	proxyAdd    = proxyAction("Add")
	proxyRemove = proxyAction("Remove")
)

type proxyControl struct {
	Action    proxyAction
	Subdomain string
	IPAddress string
	Port      int
}

type ReverseProxy struct {
	mu        sync.RWMutex
	cfg       *Config
	domainMap map[string]proxyHandlers
}

func NewReverseProxy(cfg *Config) *ReverseProxy {
	return &ReverseProxy{
		cfg:       cfg,
		domainMap: make(map[string]proxyHandlers),
	}
}

func (r *ReverseProxy) ServeHTTPWithPort(w http.ResponseWriter, req *http.Request, port int) {
	subdomain := strings.ToLower(strings.Split(req.Host, ".")[0])

	if handler := r.findHandler(subdomain, port); handler != nil {
		log.Printf("[debug] proxy handler found for subdomain %s", subdomain)
		handler.ServeHTTP(w, req)
	} else {
		log.Printf("[warn] proxy handler not found for subdomain %s", subdomain)
		http.NotFound(w, req)
	}
}

func (r *ReverseProxy) Exists(subdomain string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.domainMap[subdomain]
	if exists {
		return true
	}
	for name, _ := range r.domainMap {
		if m, _ := path.Match(name, subdomain); m {
			return true
		}
	}
	return false
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
	log.Printf("[debug] findHandler for %s:%d", subdomain, port)

	proxyHandlers, ok := r.domainMap[subdomain]
	if !ok {
		for name, ph := range r.domainMap {
			if m, _ := path.Match(name, subdomain); m {
				proxyHandlers = ph
				break
			}
		}
		if proxyHandlers == nil {
			return nil
		}
	}

	handler, ok := proxyHandlers.Handler(port)
	if !ok {
		return nil
	}
	return handler
}

type proxyHandlers map[int]map[string]http.Handler

func (ph proxyHandlers) Handler(port int) (http.Handler, bool) {
	handlers := ph[port]
	if len(handlers) == 0 {
		return nil, false
	}
	for _, handler := range ph[port] {
		return handler, true // return first (randomized by Go's map)
	}
	return nil, false
}

func (ph proxyHandlers) Exists(port int, ipaddress string) bool {
	if ph[port] == nil {
		return false
	}
	return ph[port][ipaddress] != nil
}

func (ph proxyHandlers) Add(port int, ipaddress string, h http.Handler) {
	if ph[port] == nil {
		ph[port] = make(map[string]http.Handler)
	}
	ph[port][ipaddress] = h
}

func (r *ReverseProxy) AddSubdomain(subdomain string, ipaddress string, targetPort int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Println("[info] adding subdomain:", subdomain, "to", ipaddress, ":", targetPort)

	var ph proxyHandlers
	if _ph, exists := r.domainMap[subdomain]; exists {
		ph = _ph
	} else {
		ph = make(proxyHandlers)
	}

	// create reverse proxy
	for _, v := range r.cfg.Listen.HTTP {
		if v.TargetPort != targetPort {
			continue
		}
		if ph.Exists(v.ListenPort, ipaddress) {
			continue
		}
		destUrlString := fmt.Sprintf("http://%s:%d", ipaddress, v.TargetPort)
		destUrl, _ := url.Parse(destUrlString)
		handler := rproxy.NewSingleHostReverseProxy(destUrl)
		ph.Add(v.ListenPort, ipaddress, handler)
		log.Printf("[info] add subdomain: %s:%d -> %s:%d", subdomain, v.ListenPort, ipaddress, targetPort)
	}
	r.domainMap[subdomain] = ph
}

func (r *ReverseProxy) RemoveSubdomain(subdomain string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	log.Println("[info] removing subdomain:", subdomain)
	delete(r.domainMap, subdomain)
}

func (r *ReverseProxy) Modify(action *proxyControl) {
	switch action.Action {
	case proxyAdd:
		r.AddSubdomain(action.Subdomain, action.IPAddress, action.Port)
	case proxyRemove:
		r.RemoveSubdomain(action.Subdomain)
	default:
		log.Printf("[error] unknown proxy action: %s", action.Action)
	}
}
