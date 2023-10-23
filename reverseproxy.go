package mirageecs

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	//	"github.com/acidlemon/go-dumper"
	"github.com/methane/rproxy"
)

type proxyAction string

const (
	proxyAdd    = proxyAction("Add")
	proxyRemove = proxyAction("Remove")
)

var proxyHandlerLifetime = 30 * time.Second

type proxyControl struct {
	Action    proxyAction
	Subdomain string
	IPAddress string
	Port      int
}

type ReverseProxy struct {
	mu                sync.RWMutex
	cfg               *Config
	domains           []string
	domainMap         map[string]proxyHandlers
	accessCounters    map[string]*AccessCounter
	accessCounterUnit time.Duration
}

func NewReverseProxy(cfg *Config) *ReverseProxy {
	unit := time.Minute
	if cfg.localMode {
		unit = time.Second * 10
		proxyHandlerLifetime = time.Hour * 24 * 365 * 10 // not expire
		log.Printf("[info] local mode: access counter unit=%s", unit)
	}
	return &ReverseProxy{
		cfg:               cfg,
		domainMap:         make(map[string]proxyHandlers),
		accessCounters:    make(map[string]*AccessCounter),
		accessCounterUnit: unit,
	}
}

func (r *ReverseProxy) ServeHTTPWithPort(w http.ResponseWriter, req *http.Request, port int) {
	subdomain := strings.ToLower(strings.Split(req.Host, ".")[0])

	if handler := r.FindHandler(subdomain, port); handler != nil {
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
	for _, name := range r.domains {
		if m, _ := path.Match(name, subdomain); m {
			return true
		}
	}
	return false
}

func (r *ReverseProxy) Subdomains() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ds := make([]string, len(r.domains))
	copy(ds, r.domains)
	return ds
}

func (r *ReverseProxy) FindHandler(subdomain string, port int) http.Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	log.Printf("[debug] FindHandler for %s:%d", subdomain, port)

	proxyHandlers, ok := r.domainMap[subdomain]
	if !ok {
		for _, name := range r.domains {
			if m, _ := path.Match(name, subdomain); m {
				proxyHandlers = r.domainMap[name]
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

type proxyHandler struct {
	handler http.Handler
	timer   *time.Timer
}

func newProxyHandler(h http.Handler) *proxyHandler {
	return &proxyHandler{
		handler: h,
		timer:   time.NewTimer(proxyHandlerLifetime),
	}
}

func (h *proxyHandler) alive() bool {
	select {
	case <-h.timer.C:
		return false
	default:
		return true
	}
}

func (h *proxyHandler) extend() {
	h.timer.Reset(proxyHandlerLifetime) // extend lifetime
}

type proxyHandlers map[int]map[string]*proxyHandler

func (ph proxyHandlers) Handler(port int) (http.Handler, bool) {
	handlers := ph[port]
	if len(handlers) == 0 {
		return nil, false
	}
	for ipaddress, handler := range ph[port] {
		if handler.alive() {
			// return first (randomized by Go's map)
			return handler.handler, true
		} else {
			log.Printf("[info] proxy handler to %s is dead", ipaddress)
			delete(ph[port], ipaddress)
		}
	}
	return nil, false
}

func (ph proxyHandlers) exists(port int, addr string) bool {
	if ph[port] == nil {
		return false
	}
	if h := ph[port][addr]; h == nil {
		return false
	} else if h.alive() {
		log.Printf("[debug] proxy handler to %s extends lifetime", addr)
		h.extend()
		return true
	} else {
		log.Printf("[info] proxy handler to %s is dead", addr)
		delete(ph[port], addr)
		return false
	}
}

func (ph proxyHandlers) add(port int, ipaddress string, h http.Handler) {
	if ph[port] == nil {
		ph[port] = make(map[string]*proxyHandler)
	}
	log.Printf("[info] new proxy handler to %s", ipaddress)
	ph[port][ipaddress] = newProxyHandler(h)
}

func (r *ReverseProxy) AddSubdomain(subdomain string, ipaddress string, targetPort int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	addr := net.JoinHostPort(ipaddress, strconv.Itoa(targetPort))
	log.Printf("[debug] AddSubdomain %s -> %s", subdomain, addr)
	var ph proxyHandlers
	if _ph, exists := r.domainMap[subdomain]; exists {
		ph = _ph
	} else {
		ph = make(proxyHandlers)
	}

	var counter *AccessCounter
	if c, exists := r.accessCounters[subdomain]; exists {
		counter = c
	} else {
		counter = NewAccessCounter(r.accessCounterUnit)
		r.accessCounters[subdomain] = counter
	}

	// create reverse proxy
	for _, v := range r.cfg.Listen.HTTP {
		if v.TargetPort != targetPort {
			if !r.cfg.localMode {
				log.Printf("[warn] target port %d is not defined in config.", targetPort)
				continue
			}
			// local mode allows any port
		}
		if ph.exists(v.ListenPort, addr) {
			continue
		}
		destUrlString := "http://" + addr
		destUrl, err := url.Parse(destUrlString)
		if err != nil {
			log.Printf("[error] invalid destination url: %s %s", destUrlString, err)
			continue
		}
		handler := rproxy.NewSingleHostReverseProxy(destUrl)
		handler.Transport = &Transport{
			Transport:         http.DefaultTransport,
			Counter:           counter,
			Timeout:           r.cfg.Network.ProxyTimeout,
			Subdomain:         subdomain,
			RequireAuthCookie: v.RequireAuthCookie,
		}
		ph.add(v.ListenPort, addr, handler)
		log.Printf("[info] add subdomain: %s:%d -> %s", subdomain, v.ListenPort, addr)
	}
	r.domainMap[subdomain] = ph
	for _, name := range r.domains {
		if name == subdomain {
			return
		}
	}
	r.domains = append(r.domains, subdomain)
}

func (r *ReverseProxy) RemoveSubdomain(subdomain string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	log.Println("[info] removing subdomain:", subdomain)
	delete(r.domainMap, subdomain)
	delete(r.accessCounters, subdomain)
	for i, name := range r.domains {
		if name == subdomain {
			r.domains = append(r.domains[:i], r.domains[i+1:]...)
			return
		}
	}
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

func (r *ReverseProxy) CollectAccessCounts() map[string]accessCount {
	r.mu.RLock()
	defer r.mu.RUnlock()
	counts := make(map[string]accessCount)
	for subdomain, counter := range r.accessCounters {
		counts[subdomain] = counter.Collect()
	}
	return counts
}

type Transport struct {
	Counter           *AccessCounter
	Transport         http.RoundTripper
	Timeout           time.Duration
	Subdomain         string
	RequireAuthCookie bool
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.Counter.Add()

	if t.RequireAuthCookie {
		cookie, err := req.Cookie(AuthCookieName)
		if err != nil || cookie == nil || cookie.Value != "ok" {
			log.Printf("[warn] subdomain %s %s roundtrip failed: %s", t.Subdomain, req.URL, err)
			return newForbiddenResponse(), nil
		}
	}
	if t.Timeout == 0 {
		return t.Transport.RoundTrip(req)
	}
	ctx, cancel := context.WithTimeout(req.Context(), t.Timeout)
	defer cancel()
	resp, err := t.Transport.RoundTrip(req.WithContext(ctx))
	if err == nil {
		return resp, nil
	}
	log.Printf("[warn] subdomain %s %s roundtrip failed: %s", t.Subdomain, req.URL, err)

	// timeout
	if ctx.Err() == context.DeadlineExceeded {
		return newTimeoutResponse(t.Subdomain, req.URL.String()), nil
	}
	return resp, err
}

func newTimeoutResponse(subdomain string, u string) *http.Response {
	resp := new(http.Response)
	resp.StatusCode = http.StatusGatewayTimeout
	msg := fmt.Sprintf("%s upstream timeout: %s", subdomain, u)
	resp.Body = io.NopCloser(strings.NewReader(msg))
	return resp
}

func newForbiddenResponse() *http.Response {
	resp := new(http.Response)
	resp.StatusCode = http.StatusForbidden
	resp.Body = io.NopCloser(strings.NewReader("Forbidden"))
	return resp
}
