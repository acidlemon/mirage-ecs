package mirageecs

import (
	"encoding/base64"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/fujiwara/go-amzn-oidc/validator"
	"github.com/labstack/echo/v4"
)

type Auth struct {
	Basic    *AuthMethodBasic    `yaml:"basic"`
	Token    *AuthMethodToken    `yaml:"token"`
	AmznOIDC *AuthMethodAmznOIDC `yaml:"amzn_oidc"`
}

func (a *Auth) Do(req *http.Request, res http.ResponseWriter) (bool, error) {
	if a == nil {
		// no auth
		return true, nil
	}
	if a.Token != nil {
		log.Println("[debug] auth token")
		if ok := a.Token.Match(req.Header); ok {
			return ok, nil
		}
	}
	if a.AmznOIDC != nil {
		log.Println("[debug] auth amzn_oidc")
		if ok, err := a.AmznOIDC.Match(req.Header); err != nil {
			return false, err
		} else if ok {
			return true, nil
		}
	}
	// basic auth is evaluated at last
	// because www-authenticate header is set if auth failed.
	if a.Basic != nil {
		log.Println("[debug] auth basic")
		if ok := a.Basic.Match(req.Header); ok {
			return ok, nil
		} else {
			res.Header().Set("WWW-Authenticate", "Basic realm=\"Restricted\"")
		}
	}
	return false, nil
}

type AuthMethodBasic struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`

	gen      sync.Once
	expected string
}

func (b *AuthMethodBasic) Match(h http.Header) bool {
	if b.Username == "" || b.Password == "" {
		return false
	}
	b.gen.Do(func() {
		b.expected = "Basic " + base64.StdEncoding.EncodeToString([]byte(b.Username+":"+b.Password))
	})
	log.Println("[debug] auth basic comparing", b.Username, b.Password, h.Get("Authorization"))
	return h.Get("Authorization") == b.expected
}

type AuthMethodToken struct {
	Token  string `yaml:"token"`
	Header string `yaml:"header"`
}

func (b *AuthMethodToken) Match(h http.Header) bool {
	if b.Token == "" {
		return false
	}
	log.Println("[debug] auth token comparing", b.Header, b.Token, h.Get(b.Header))
	return h.Get(b.Header) == b.Token
}

type AuthMethodAmznOIDC struct {
	Claim    string          `yaml:"claim"` // e.g. "email" see alsohttps://openid.net/specs/openid-connect-core-1_0.html#StandardClaims
	Matchers []*ClaimMatcher `yaml:"matchers"`
}

func (a *AuthMethodAmznOIDC) Match(h http.Header) (bool, error) {
	if a.Claim == "" {
		return false, nil
	}
	log.Printf("[debug] auth amzn_oidc comparing %s with %s", a.Claim, h.Get("x-amzn-oidc-data"))
	claims, err := validator.Validate(h.Get("x-amzn-oidc-data"))
	if err != nil {
		return false, err
	}
	for _, m := range a.Matchers {
		log.Printf("[debug] auth amzn_oidc matching %s", *m)
		v, ok := claims[a.Claim]
		if !ok {
			log.Printf("[warn] auth amzn_oidc claim[%s] not found in claims", a.Claim)
			return false, nil
		}
		switch v := v.(type) {
		case string:
			return m.Match(v), nil
		default:
			log.Printf("[warn] auth amzn_oidc claim[%s] is not a string: %v", a.Claim, v)
			return false, nil
		}
	}
	return false, nil
}

func (cfg *Config) AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ok, err := cfg.Auth.Do(c.Request(), c.Response())
		if err != nil {
			log.Println("[error] auth error:", err)
			return echo.ErrInternalServerError
		}
		if !ok {
			log.Println("[warn] auth failed")
			return echo.ErrUnauthorized
		}
		return next(c)
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
