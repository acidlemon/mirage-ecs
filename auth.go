package mirageecs

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fujiwara/go-amzn-oidc/validator"
	"github.com/golang-jwt/jwt/v4"
)

type Auth struct {
	Basic        *AuthMethodBasic    `yaml:"basic"`
	Token        *AuthMethodToken    `yaml:"token"`
	AmznOIDC     *AuthMethodAmznOIDC `yaml:"amzn_oidc"`
	CookieSecret string              `yaml:"cookie_secret"`

	jwtParser  *jwt.Parser
	jwtKeyFunc func(*jwt.Token) (interface{}, error)
	once       sync.Once
}

type Authorizer func(req *http.Request, res http.ResponseWriter) (bool, error)

func (a *Auth) ByBasic(req *http.Request, res http.ResponseWriter) (bool, error) {
	if a == nil || a.Basic == nil {
		return false, nil
	}
	if ok := a.Basic.Match(req.Header); ok {
		slog.Debug("basic auth succeeded")
		return ok, nil
	} else {
		slog.Debug("basic auth failed. set WWW-Authenticate header")
		res.Header().Set("WWW-Authenticate", "Basic realm=\"Restricted\"")
	}
	return false, nil
}

func (a *Auth) ByToken(req *http.Request, res http.ResponseWriter) (bool, error) {
	if a == nil || a.Token == nil {
		return false, nil
	}
	if ok := a.Token.Match(req.Header); ok {
		slog.Debug("token auth succeeded")
		return ok, nil
	}
	slog.Debug("token auth failed")
	return false, nil
}

func (a *Auth) ByAmznOIDC(req *http.Request, res http.ResponseWriter) (bool, error) {
	if a == nil || a.AmznOIDC == nil {
		return false, nil
	}
	if ok, err := a.AmznOIDC.Match(req.Header); err != nil {
		return false, err
	} else if ok {
		slog.Debug("amzn_oidc auth succeeded")
		return true, nil
	}
	slog.Debug("amzn_oidc auth failed")
	return false, nil
}

func (a *Auth) Do(req *http.Request, res http.ResponseWriter, runs ...Authorizer) (bool, error) {
	if a == nil {
		// no auth
		return true, nil
	}
	for _, run := range runs {
		if ok, err := run(req, res); err != nil {
			return false, fmt.Errorf("authorizer %v errored: %w", run, err)
		} else if ok {
			return true, nil
		}
	}
	return false, nil
}

func (a *Auth) NewAuthCookie(expire time.Duration, domain string) (*http.Cookie, error) {
	expireAt := time.Now().Add(expire)

	if a == nil || a.CookieSecret == "" {
		return &http.Cookie{}, nil
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"expire_at": expireAt.Unix(),
	})
	tokenStr, err := token.SignedString([]byte(a.CookieSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign cookie: %w", err)
	}
	return &http.Cookie{
		Name:     AuthCookieName,
		Value:    tokenStr,
		Expires:  expireAt,
		Domain:   domain,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	}, nil
}

func (a *Auth) ValidateAuthCookie(c *http.Cookie) error {
	if a == nil || a.CookieSecret == "" {
		return fmt.Errorf("cookie_secret is not set")
	}
	a.once.Do(func() {
		a.jwtParser = jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))
		a.jwtKeyFunc = func(token *jwt.Token) (interface{}, error) {
			return []byte(a.CookieSecret), nil
		}
	})
	token, err := a.jwtParser.Parse(c.Value, a.jwtKeyFunc)
	if err != nil {
		return fmt.Errorf("failed to parse cookie: %w", err)
	}
	if !token.Valid {
		return fmt.Errorf("invalid cookie: %v", token)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return fmt.Errorf("invalid claims: %v", token.Claims)
	}
	expireAt, ok := claims["expire_at"].(float64)
	if !ok {
		return fmt.Errorf("invalid expire_at: %v", claims["expire_at"])
	}
	if time.Now().Unix() >= int64(expireAt) {
		return fmt.Errorf("already expired: %v", expireAt)
	}
	return nil
}

type AuthMethodBasic struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`

	gen      sync.Once
	expected string
}

func (b *AuthMethodBasic) Match(h http.Header) bool {
	if b == nil {
		return false
	}
	slog.Debug(f("auth basic %s %s", b.Username, b.Password))
	if b.Username == "" || b.Password == "" || h.Get("Authorization") == "" {
		return false
	}
	b.gen.Do(func() {
		b.expected = "Basic " + base64.StdEncoding.EncodeToString([]byte(b.Username+":"+b.Password))
	})
	slog.Debug(f("auth basic comparing %s %s %s", b.Username, b.Password, h.Get("Authorization")))
	if h.Get("Authorization") == b.expected {
		slog.Debug(f("auth basic succeeded"))
		return true
	}
	slog.Warn("auth basic failed")
	return false
}

type AuthMethodToken struct {
	Token  string `yaml:"token"`
	Header string `yaml:"header"`
}

func (b *AuthMethodToken) Match(h http.Header) bool {
	if b == nil {
		return false
	}
	sent := h.Get(b.Header)
	if b.Token == "" || sent == "" {
		return false
	}
	slog.Debug(f("auth token comparing %s %s %s", b.Header, b.Token, sent))
	if b.Token == sent {
		slog.Debug("auth token succeeded")
		return true
	}
	slog.Warn(f("auth token (header=%s) does not match", b.Header))
	return false
}

type AuthMethodAmznOIDC struct {
	Claim    string          `yaml:"claim"` // e.g. "email" see alsohttps://openid.net/specs/openid-connect-core-1_0.html#StandardClaims
	Matchers []*ClaimMatcher `yaml:"matchers"`
}

func (a *AuthMethodAmznOIDC) Match(h http.Header) (bool, error) {
	if a == nil {
		return false, nil
	}
	if a.Claim == "" {
		return false, nil
	}
	slog.Debug(f("auth amzn_oidc comparing %s with %s", a.Claim, h.Get("x-amzn-oidc-data")))
	claims, err := validator.Validate(h.Get("x-amzn-oidc-data"))
	if err != nil {
		return false, fmt.Errorf("failed to validate x-amzn-oidc-data: %s", err)
	}
	return a.MatchClaims(claims), nil
}

func (a *AuthMethodAmznOIDC) MatchClaims(claims map[string]interface{}) bool {
	v, ok := claims[a.Claim]
	if !ok {
		slog.Warn(f("auth amzn_oidc claim[%s] not found in claims", a.Claim))
		return false
	}
	vs, ok := v.(string)
	if !ok {
		slog.Warn(f("auth amzn_oidc claim[%s] is not a string: %v", a.Claim, v))
		return false
	}
	for _, m := range a.Matchers {
		if m.Match(vs) {
			slog.Debug(f("auth amzn_oidc claim[%s]=%s matches %#v", a.Claim, v, m))
			return true
		}
		slog.Debug(f("auth amzn_oidc claim[%s]=%s does not match %#v", a.Claim, v, m))
	}
	slog.Warn(f("auth amzn_oidc claim[%s]=%s does not match any matchers", a.Claim, vs))
	return false
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
