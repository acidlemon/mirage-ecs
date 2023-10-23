package mirageecs_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mirageecs "github.com/acidlemon/mirage-ecs"
)

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name              string
		serverDelay       time.Duration
		timeout           time.Duration
		wantStatus        int
		wantBody          string
		requireAuthCookie bool
		sendCookie        bool
	}{
		{
			name:        "Success pattern",
			serverDelay: 50 * time.Millisecond,
			timeout:     100 * time.Millisecond,
			wantStatus:  http.StatusOK,
			wantBody:    "OK",
		},
		{
			name:        "Timeout failure pattern",
			serverDelay: 150 * time.Millisecond,
			timeout:     100 * time.Millisecond,
			wantStatus:  http.StatusGatewayTimeout,
			wantBody:    "test-subdomain upstream timeout: ",
		},
		{
			name:              "Success pattern with auth cookie",
			timeout:           100 * time.Millisecond,
			wantStatus:        http.StatusOK,
			wantBody:          "OK",
			requireAuthCookie: true,
			sendCookie:        true,
		},
		{
			name:              "Forbidden pattern with auth cookie",
			timeout:           100 * time.Millisecond,
			wantStatus:        http.StatusForbidden,
			wantBody:          "Forbidden",
			requireAuthCookie: true,
			sendCookie:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(tt.serverDelay)
				w.Write([]byte("OK"))
			}))
			defer server.Close()

			// Setup transport
			tr := &mirageecs.Transport{
				Counter:           mirageecs.NewAccessCounter(time.Second),
				Transport:         http.DefaultTransport,
				Timeout:           tt.timeout,
				Subdomain:         "test-subdomain",
				RequireAuthCookie: tt.requireAuthCookie,
			}

			req, _ := http.NewRequest("GET", server.URL, nil)
			if tt.sendCookie {
				req.AddCookie(&http.Cookie{
					Name:  "mirage-ecs-auth",
					Value: "ok",
				})
			}

			resp, err := tr.RoundTrip(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("wanted status %v, got %v", tt.wantStatus, resp.StatusCode)
			}

			if !strings.Contains(string(body), tt.wantBody) {
				t.Errorf("wanted body to contain %v, got %v", tt.wantBody, string(body))
			}
		})
	}
}
