package mirageecs

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
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
		slog.Debug(f("local mode: access counter unit=%s", unit))
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
		slog.Debug(f("proxy handler found for subdomain %s", subdomain))
		handler.ServeHTTP(w, req)
	} else {
		slog.Debug(f("proxy handler not found for subdomain %s", subdomain))
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
	slog.Debug(f("FindHandler for %s:%d", subdomain, port))

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
			slog.Info(f("proxy handler to %s is dead", ipaddress))
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
		slog.Debug(f("proxy handler to %s extends lifetime", addr))
		h.extend()
		return true
	} else {
		slog.Info(f("proxy handler to %s is dead", addr))
		delete(ph[port], addr)
		return false
	}
}

func (ph proxyHandlers) add(port int, ipaddress string, h http.Handler) {
	if ph[port] == nil {
		ph[port] = make(map[string]*proxyHandler)
	}
	slog.Info(f("new proxy handler to %s", ipaddress))
	ph[port][ipaddress] = newProxyHandler(h)
}

func (r *ReverseProxy) AddSubdomain(subdomain string, ipaddress string, targetPort int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	addr := net.JoinHostPort(ipaddress, strconv.Itoa(targetPort))
	slog.Debug(f("AddSubdomain %s -> %s", subdomain, addr))
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
	proxy := false
	for _, v := range r.cfg.Listen.HTTP {
		if (v.TargetPort != targetPort) && !r.cfg.localMode {
			continue
			// local mode allows any port
		}
		if ph.exists(v.ListenPort, addr) {
			proxy = true
			continue
		}
		destUrlString := "http://" + addr
		destUrl, err := url.Parse(destUrlString)
		if err != nil {
			slog.Error(f("invalid destination url: %s %s", destUrlString, err))
			continue
		}
		tp := &Transport{
			Transport: newHTTPTransport(r.cfg.Network.ProxyTimeout),
			Counter:   counter,
			Subdomain: subdomain,
		}
		if v.RequireAuthCookie {
			tp.AuthCookieValidateFunc = r.cfg.Auth.ValidateAuthCookie
		}
		handler := newUpstreamProxy(destUrl, tp)
		ph.add(v.ListenPort, addr, handler)
		proxy = true
		slog.Info(f("add subdomain: %s:%d -> %s", subdomain, v.ListenPort, addr))
	}
	if !proxy {
		slog.Warn(f("proxy of subdomain %s(target port %d) is not created. define target port in listen.http[]", subdomain, targetPort))
		return
	}

	r.domainMap[subdomain] = ph
	if slices.Contains(r.domains, subdomain) {
		return
	}
	r.domains = append(r.domains, subdomain)
}

func (r *ReverseProxy) RemoveSubdomain(subdomain string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	slog.Info(f("removing subdomain: %s", subdomain))
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
		slog.Error(f("unknown proxy action: %s", action.Action))
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

// forwardedHeaders are the headers which httputil.ReverseProxy removes from
// the outbound request before calling Rewrite.
var forwardedHeaders = []string{
	"Forwarded",
	"X-Forwarded-For",
	"X-Forwarded-Host",
	"X-Forwarded-Proto",
}

// newUpstreamProxy creates a reverse proxy to the target.
//
// The proxy passes requests as transparently as possible: the Host header
// and the raw query are not modified, and the forwarded headers of the
// inbound request are preserved because mirage-ecs is supposed to run behind
// a load balancer which sets them. The client address is appended to
// X-Forwarded-For.
//
// The forwarded headers are preserved even if the inbound Connection header
// nominates them as hop-by-hop, otherwise a client could remove the values
// set by the load balancer.
func newUpstreamProxy(target *url.URL, tp http.RoundTripper) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = pr.In.Host
			pr.Out.URL.RawQuery = pr.In.URL.RawQuery
			for _, name := range forwardedHeaders {
				if v, ok := pr.In.Header[name]; ok {
					pr.Out.Header[name] = slices.Clone(v)
				}
			}
			if clientIP, _, err := net.SplitHostPort(pr.In.RemoteAddr); err == nil {
				if prior := pr.Out.Header["X-Forwarded-For"]; len(prior) > 0 {
					clientIP = strings.Join(prior, ", ") + ", " + clientIP
				}
				pr.Out.Header.Set("X-Forwarded-For", clientIP)
			}
		},
		Transport: tp,
		// Flush immediately after each write to the client, so that
		// streaming responses are delivered in real time.
		FlushInterval: -1,
	}
}

func newHTTPTransport(t time.Duration) http.RoundTripper {
	tp := http.DefaultTransport.(*http.Transport).Clone()
	tp.DialContext = (&net.Dialer{
		Timeout:   t,
		KeepAlive: 30 * time.Second,
	}).DialContext
	tp.TLSHandshakeTimeout = t
	tp.ResponseHeaderTimeout = t
	return tp
}

type Transport struct {
	Counter                *AccessCounter
	Transport              http.RoundTripper
	Subdomain              string
	AuthCookieValidateFunc func(*http.Cookie) error
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.Counter.Add()

	slog.Debug(f("subdomain %s %s roundtrip", t.Subdomain, req.URL))
	// OPTIONS request is not authenticated because it is preflighted.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Access_control_CORS#Preflighted_requests
	if t.AuthCookieValidateFunc != nil && req.Method != http.MethodOptions {
		slog.Debug(f("subdomain %s %s roundtrip: require auth cookie", t.Subdomain, req.URL))
		cookie, err := req.Cookie(AuthCookieName)
		if err != nil || cookie == nil {
			slog.Warn(f("subdomain %s %s roundtrip failed: %s", t.Subdomain, req.URL, err))
			return newForbiddenResponse(), nil
		}
		if err := t.AuthCookieValidateFunc(cookie); err != nil {
			slog.Warn(f("subdomain %s %s roundtrip failed: %s", t.Subdomain, req.URL, err))
			return newForbiddenResponse(), nil
		}
	}
	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		slog.Warn(f("subdomain %s %s roundtrip failed: %s", t.Subdomain, req.URL, err))
		if strings.Contains(err.Error(), "timeout") {
			return newTimeoutResponse(t.Subdomain, req.URL.String(), err), nil
		}
		return nil, err
	}
	return resp, nil
}

func newTimeoutResponse(subdomain string, u string, err error) *http.Response {
	resp := new(http.Response)
	resp.StatusCode = http.StatusGatewayTimeout
	msg := fmt.Sprintf("%s upstream timeout: %s %s", subdomain, u, err.Error())
	resp.Body = io.NopCloser(strings.NewReader(msg))
	return resp
}

func newForbiddenResponse() *http.Response {
	resp := new(http.Response)
	resp.StatusCode = http.StatusForbidden
	resp.Body = io.NopCloser(strings.NewReader("Forbidden"))
	return resp
}
