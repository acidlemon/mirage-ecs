package mirageecs_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	mirageecs "github.com/acidlemon/mirage-ecs/v2"
)

// newTestProxy returns a server which proxies all requests to upstream
// as the subdomain "app".
func newTestProxy(t *testing.T, upstreamAddr string, requireAuthCookie bool) *httptest.Server {
	t.Helper()
	return newTestProxyWithTimeout(t, upstreamAddr, requireAuthCookie, 0)
}

func newTestProxyWithTimeout(t *testing.T, upstreamAddr string, requireAuthCookie bool, timeout time.Duration) *httptest.Server {
	t.Helper()
	host, portStr, err := net.SplitHostPort(upstreamAddr)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portStr)
	cfg, err := mirageecs.NewConfig(context.Background(), &mirageecs.ConfigParams{Domain: "example.net"})
	if err != nil {
		t.Fatal(err)
	}
	cfg.Listen.HTTP = []mirageecs.PortMap{
		{ListenPort: 80, TargetPort: port, RequireAuthCookie: requireAuthCookie},
	}
	cfg.Network.ProxyTimeout = timeout
	rp := mirageecs.NewReverseProxy(cfg)
	rp.AddSubdomain("app", host, port)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = "app.example.net"
		rp.ServeHTTPWithPort(w, r, 80)
	}))
	t.Cleanup(proxy.Close)
	return proxy
}

func TestReverseProxyRequest(t *testing.T) {
	var got *http.Request
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Header().Set("X-Upstream", "yes")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, "created")
	}))
	t.Cleanup(upstream.Close)
	proxy := newTestProxy(t, upstream.Listener.Addr().String(), false)

	// a raw query which contains a semicolon must be passed as is.
	req, _ := http.NewRequest(http.MethodPost, proxy.URL+"/path/to?a=1;b=2&c=%20", strings.NewReader("body"))
	// headers set by a load balancer in front of mirage-ecs
	req.Header.Set("X-Forwarded-For", "192.0.2.1")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Port", "443")
	req.Header.Set("X-Forwarded-Host", "app.example.net")
	req.Header.Set("X-Custom", "custom")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if string(body) != "created" {
		t.Errorf("body = %q, want %q", body, "created")
	}
	if v := resp.Header.Get("X-Upstream"); v != "yes" {
		t.Errorf("response header X-Upstream = %q, want %q", v, "yes")
	}

	if got.Host != "app.example.net" {
		t.Errorf("Host = %q, want %q", got.Host, "app.example.net")
	}
	if got.URL.Path != "/path/to" {
		t.Errorf("path = %q, want %q", got.URL.Path, "/path/to")
	}
	if got.URL.RawQuery != "a=1;b=2&c=%20" {
		t.Errorf("query = %q, want %q", got.URL.RawQuery, "a=1;b=2&c=%20")
	}
	for name, want := range map[string]string{
		"X-Forwarded-For":   "192.0.2.1, 127.0.0.1",
		"X-Forwarded-Proto": "https",
		"X-Forwarded-Port":  "443",
		"X-Forwarded-Host":  "app.example.net",
		"X-Custom":          "custom",
	} {
		if v := got.Header.Get(name); v != want {
			t.Errorf("request header %s = %q, want %q", name, v, want)
		}
	}
}

func TestReverseProxyAuthCookie(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "OK")
	}))
	t.Cleanup(upstream.Close)
	proxy := newTestProxy(t, upstream.Listener.Addr().String(), true)

	resp, err := http.Get(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	if string(body) != "Forbidden" {
		t.Errorf("body = %q, want %q", body, "Forbidden")
	}
}

func TestReverseProxyTimeout(t *testing.T) {
	done := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select { // never respond
		case <-done:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(done); upstream.Close() })
	proxy := newTestProxyWithTimeout(t, upstream.Listener.Addr().String(), false, 100*time.Millisecond)

	resp, err := http.Get(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusGatewayTimeout {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusGatewayTimeout)
	}
	if !strings.Contains(string(body), "app upstream timeout: ") {
		t.Errorf("body = %q, want to contain %q", body, "app upstream timeout: ")
	}
}

func TestReverseProxyUpstreamDown(t *testing.T) {
	// find an address nobody listens on
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	proxy := newTestProxy(t, addr, false)

	resp, err := http.Get(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadGateway)
	}
}

// A streaming response must reach the client without waiting for the
// end of the response, regardless of its Content-Type.
func TestReverseProxyStreaming(t *testing.T) {
	for _, contentType := range []string{"text/event-stream", "application/x-ndjson"} {
		t.Run(contentType, func(t *testing.T) {
			done := make(chan struct{})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", contentType)
				io.WriteString(w, "data: hello\n\n")
				w.(http.Flusher).Flush()
				select { // keep the response open
				case <-done:
				case <-r.Context().Done():
				}
			}))
			t.Cleanup(func() { close(done); upstream.Close() })
			proxy := newTestProxy(t, upstream.Listener.Addr().String(), false)

			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Get(proxy.URL)
			if err != nil {
				t.Fatalf("response headers are not delivered: %s", err)
			}
			defer resp.Body.Close()
			line, err := bufio.NewReader(resp.Body).ReadString('\n')
			if err != nil {
				t.Fatalf("first chunk is not delivered: %s", err)
			}
			if line != "data: hello\n" {
				t.Errorf("first chunk = %q, want %q", line, "data: hello\n")
			}
		})
	}
}

// newUpgradeServer returns the address of a server which accepts an
// Upgrade request and then echoes lines with the prefix "echo: ".
func newUpgradeServer(t *testing.T) string {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			http.Error(w, "upgrade required", http.StatusUpgradeRequired)
			return
		}
		conn, brw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		io.WriteString(brw, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
		brw.Flush()
		for {
			line, err := brw.ReadString('\n')
			if err != nil {
				return
			}
			io.WriteString(brw, "echo: "+line)
			brw.Flush()
		}
	}))
	t.Cleanup(upstream.Close)
	return upstream.Listener.Addr().String()
}

func TestReverseProxyUpgrade(t *testing.T) {
	tests := []struct {
		name              string
		requireAuthCookie bool
		wantStatus        int
	}{
		{name: "upgraded", wantStatus: http.StatusSwitchingProtocols},
		{name: "forbidden without auth cookie", requireAuthCookie: true, wantStatus: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := newTestProxy(t, newUpgradeServer(t), tt.requireAuthCookie)

			conn, err := net.Dial("tcp", proxy.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(3 * time.Second))
			fmt.Fprint(conn, "GET /ws HTTP/1.1\r\nHost: app.example.net\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
			br := bufio.NewReader(conn)
			resp, err := http.ReadResponse(br, nil)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusSwitchingProtocols {
				return
			}
			for _, msg := range []string{"hello", "world"} {
				fmt.Fprintln(conn, msg)
				line, err := br.ReadString('\n')
				if err != nil {
					t.Fatal(err)
				}
				if want := "echo: " + msg + "\n"; line != want {
					t.Errorf("got %q, want %q", line, want)
				}
			}
		})
	}
}
