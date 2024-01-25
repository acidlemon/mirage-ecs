package mirageecs_test

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mirageecs "github.com/acidlemon/mirage-ecs/v2"
	"github.com/labstack/echo/v4"
)

func TestAuthMiddleware(t *testing.T) {
	config := &mirageecs.Config{
		Host: mirageecs.Host{
			WebApi:             "mirage.localtest.me",
			ReverseProxySuffix: ".localtest.me",
		},
		Auth: &mirageecs.Auth{
			CookieSecret: "cookie-secret",
			Token: &mirageecs.AuthMethodToken{
				Header: "x-mirage-token",
				Token:  "mytoken",
			},
			Basic: &mirageecs.AuthMethodBasic{
				Username: "user",
				Password: "pass",
			},
		},
	}

	var validCookie *http.Cookie
	validateCookie := func(cookies []*http.Cookie) error {
		for _, c := range cookies {
			if c.Name == mirageecs.AuthCookieName && c.HttpOnly && c.Secure && c.Domain == "localtest.me" {
				validCookie = c
				return nil
			}
		}
		return fmt.Errorf("cookie is not set correctly %v", cookies)
	}

	cases := []struct {
		Name         string
		Request      func() *http.Request
		ExpectStatus int
		Expect       func(*http.Response) error
		BodyContains string
	}{
		{
			Name: "Token matches",
			Request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("x-mirage-token", "mytoken")
				return req
			},
			ExpectStatus: 200,
			Expect: func(res *http.Response) error {
				if err := validateCookie(res.Cookies()); err != nil {
					return err
				}
				return nil
			},
			BodyContains: "ok",
		},
		{
			Name: "Token does not match",
			Request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("x-mirage-token", "othertoken")
				return req
			},
			ExpectStatus: 401,
			Expect: func(res *http.Response) error {
				if res.Header.Get("WWW-Authenticate") != `Basic realm="Restricted"` {
					return fmt.Errorf("WWW-Authenticate header is not set")
				}
				if len(res.Cookies()) != 0 {
					return fmt.Errorf("cookie should not be set %v", res.Cookies())
				}
				return nil
			},
		},
		{
			Name: "Basic matches",
			Request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.SetBasicAuth("user", "pass")
				return req
			},
			ExpectStatus: 200,
			Expect: func(res *http.Response) error {
				if err := validateCookie(res.Cookies()); err != nil {
					return err
				}
				return nil
			},
			BodyContains: "ok",
		},
		{
			Name: "Basic does not match",
			Request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.SetBasicAuth("userx", "pass")
				return req
			},
			ExpectStatus: 401,
			Expect: func(res *http.Response) error {
				if len(res.Cookies()) != 0 {
					return fmt.Errorf("cookie should not be set %v", res.Cookies())
				}
				return nil
			},
		},
		{
			Name: "token matches, basic does not match",
			Request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("x-mirage-token", "mytoken")
				req.SetBasicAuth("userx", "pass")
				return req
			},
			ExpectStatus: 200,
			Expect: func(res *http.Response) error {
				if err := validateCookie(res.Cookies()); err != nil {
					return err
				}
				return nil
			},
			BodyContains: "ok",
		},
		{
			Name: "/api/* don't allow basic auth",
			Request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/api/list", nil)
				req.SetBasicAuth("user", "pass")
				return req
			},
			ExpectStatus: 401,
			Expect: func(res *http.Response) error {
				if len(res.Cookies()) != 0 {
					return fmt.Errorf("cookie should not be set %v", res.Cookies())
				}
				return nil
			},
		},
		{
			Name: "POST /launch requries origin header",
			Request: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/launch", nil)
				req.SetBasicAuth("user", "pass")
				return req
			},
			ExpectStatus: 400,
		},
		{
			Name: "POST /launch succeeds with valid origin header with basic auth",
			Request: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/launch", nil)
				req.SetBasicAuth("user", "pass")
				req.Header.Set("Origin", "https://mirage.localtest.me:8000")
				return req
			},
			ExpectStatus: 200,
			BodyContains: "launched",
		},
		{
			Name: "POST /launch fails with valid origin header with valid cookie only",
			Request: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/launch", nil)
				req.AddCookie(validCookie)
				req.Header.Set("Origin", "https://mirage.localtest.me:8000")
				return req
			},
			ExpectStatus: 401,
		},
		{
			Name: "webapi dosen't allow cookie auth",
			Request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/api/list", nil)
				req.AddCookie(validCookie)
				return req
			},
			ExpectStatus: 401,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			e := echo.New()
			handler := func(c echo.Context) error {
				p := c.Request().URL.Path
				log.Printf("[debug] handler called %s", p)
				switch p {
				case "/":
					return c.String(http.StatusOK, "ok")
				case "/api/list":
					return c.JSON(http.StatusOK, []int{})
				case "/launch":
					log.Printf("[debug] origin: %s", c.Request().Header.Get("Origin"))
					if err := config.ValidateOrigin(c.Request().Header.Get("Origin")); err != nil {
						log.Printf("[debug] origin validation failed: %s", err)
						return c.String(http.StatusBadRequest, err.Error())
					}
					return c.String(http.StatusOK, "launched")
				default:
					return c.String(http.StatusNotFound, "not found")
				}
			}
			middleware := config.AuthMiddleware(handler)
			rec := httptest.NewRecorder()
			c := e.NewContext(tc.Request(), rec)
			err := middleware(c)
			if err == nil {
				if rec.Code != tc.ExpectStatus {
					t.Errorf("unexpected status code: %d", rec.Code)
				}
			} else {
				if code := err.(*echo.HTTPError).Code; code != tc.ExpectStatus {
					t.Errorf("unexpected status code: %d", code)
				}
			}
			if tc.Expect != nil {
				if err := tc.Expect(rec.Result()); err != nil {
					t.Error(err)
				}
			}
			if tc.BodyContains != "" {
				if !strings.Contains(rec.Body.String(), tc.BodyContains) {
					t.Errorf("body does not contain %s got: %s", tc.BodyContains, rec.Body.String())
				}
			}
		})
	}
}
