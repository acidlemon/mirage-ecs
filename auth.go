package mirageecs

import (
	"encoding/base64"
	"log"
	"strings"
	"sync"

	"github.com/fujiwara/go-amzn-oidc/validator"
	"github.com/labstack/echo/v4"
)

type Auth struct {
	Basic    *AuthMethodBasic    `yaml:"basic"`
	Bearer   *AuthMethodBearer   `yaml:"bearer"`
	AmznOIDC *AuthMethodAmznOIDC `yaml:"amzn_oidc"`
}

type AuthMethodBasic struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`

	gen      sync.Once
	expected string
}

func (b *AuthMethodBasic) Match(c echo.Context) bool {
	if b.Username == "" || b.Password == "" {
		return false
	}
	b.gen.Do(func() {
		b.expected = "Basic " + base64.StdEncoding.EncodeToString([]byte(b.Username+":"+b.Password))
	})
	log.Println("[debug] auth basic comparing", b.Username, b.Password, c.Request().Header.Get("Authorization"))
	return c.Request().Header.Get("Authorization") == b.expected
}

type AuthMethodBearer struct {
	Token  string `yaml:"token"`
	Header string `yaml:"header"`
}

func (b *AuthMethodBearer) Match(c echo.Context) bool {
	if b.Token == "" {
		return false
	}
	log.Println("[debug] auth bearer comparing", b.Header, b.Token, c.Request().Header.Get(b.Header))
	return c.Request().Header.Get(b.Header) == b.Token
}

type AuthMethodAmznOIDC struct {
	Claim    string          `yaml:"claim"` // e.g. "email" see alsohttps://openid.net/specs/openid-connect-core-1_0.html#StandardClaims
	Matchers []*ClaimMatcher `yaml:"matchers"`
}

func (a *AuthMethodAmznOIDC) Match(c echo.Context) bool {
	if a.Claim == "" {
		return false
	}
	log.Printf("[debug] auth amzn_oidc comparing %s with %s", a.Claim, c.Request().Header.Get("x-amzn-oidc-data"))
	claims, err := validator.Validate(c.Request().Header.Get("x-amzn-oidc-data"))
	if err != nil {
		return false
	}
	for _, m := range a.Matchers {
		log.Printf("[debug] auth amzn_oidc matching %s", *m)
		v, ok := claims[a.Claim]
		if !ok {
			log.Printf("[warn] auth amzn_oidc claim[%s] not found in claims", a.Claim)
			return false
		}
		switch v := v.(type) {
		case string:
			return m.Match(v)
		default:
			return false
		}
	}
	return false
}

func (a *Auth) AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if a.Bearer != nil {
			log.Println("[debug] auth bearer")
			if a.Bearer.Match(c) {
				return next(c)
			}
		}
		if a.AmznOIDC != nil {
			log.Println("[debug] auth amzn_oidc")
			if a.AmznOIDC.Match(c) {
				return next(c)
			}
		}
		// basic auth is evaluated at last
		// because www-authenticate header is set if auth failed.
		if a.Basic != nil {
			log.Println("[debug] auth basic")
			if a.Basic.Match(c) {
				return next(c)
			}
			c.Response().Header().Set("WWW-Authenticate", "Basic realm=\"Restricted\"")
		}
		return echo.ErrUnauthorized
	}
}

type ClaimMatcher struct {
	Exact  string `yaml:"exact"`
	Suffix string `yaml:"suffix"`
}

func (m *ClaimMatcher) Match(s string) bool {
	if m.Exact != "" {
		return m.Exact == s
	} else if m.Suffix != "" {
		return strings.HasSuffix(s, m.Suffix)
	} else {
		return false
	}
}
