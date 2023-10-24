package mirageecs

import (
	"encoding/base64"
	"fmt"
	"log"
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

func (a *Auth) Do(req *http.Request, res http.ResponseWriter) (bool, error) {
	if a == nil {
		// no auth
		return true, nil
	}

	if cookie, err := req.Cookie(AuthCookieName); err == nil {
		if err := a.ValidateAuthCookie(cookie); err != nil {
			log.Printf("[warn] auth cookie failed: %s", err)
			// fallthrough
		} else {
			log.Println("[debug] auth cookie succeeded")
			return true, nil
		}
	}

	if ok := a.Token.Match(req.Header); ok {
		return ok, nil
	}
	if ok, err := a.AmznOIDC.Match(req.Header); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}
	// basic auth is evaluated at last
	// because www-authenticate header is set if auth failed.
	if ok := a.Basic.Match(req.Header); ok {
		return ok, nil
	} else {
		res.Header().Set("WWW-Authenticate", "Basic realm=\"Restricted\"")
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
	log.Println("[debug] auth basic", b.Username, b.Password)
	if b.Username == "" || b.Password == "" || h.Get("Authorization") == "" {
		return false
	}
	b.gen.Do(func() {
		b.expected = "Basic " + base64.StdEncoding.EncodeToString([]byte(b.Username+":"+b.Password))
	})
	log.Println("[debug] auth basic comparing", b.Username, b.Password, h.Get("Authorization"))
	if h.Get("Authorization") == b.expected {
		log.Println("[debug] auth basic succeeded")
		return true
	}
	log.Printf("[warn] auth basic failed")
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
	log.Println("[debug] auth token comparing", b.Header, b.Token, sent)
	if b.Token == sent {
		log.Println("[debug] auth token succeeded")
		return true
	}
	log.Printf("[warn] auth token (header=%s) does not match", b.Header)
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
	log.Printf("[debug] auth amzn_oidc comparing %s with %s", a.Claim, h.Get("x-amzn-oidc-data"))
	claims, err := validator.Validate(h.Get("x-amzn-oidc-data"))
	if err != nil {
		return false, fmt.Errorf("failed to validate x-amzn-oidc-data: %s", err)
	}
	return a.MatchClaims(claims), nil
}

func (a *AuthMethodAmznOIDC) MatchClaims(claims map[string]interface{}) bool {
	v, ok := claims[a.Claim]
	if !ok {
		log.Printf("[warn] auth amzn_oidc claim[%s] not found in claims", a.Claim)
		return false
	}
	vs, ok := v.(string)
	if !ok {
		log.Printf("[warn] auth amzn_oidc claim[%s] is not a string: %v", a.Claim, v)
		return false
	}
	for _, m := range a.Matchers {
		if m.Match(vs) {
			log.Printf("[debug] auth amzn_oidc claim[%s]=%s matches %#v", a.Claim, v, m)
			return true
		}
		log.Printf("[debug] auth amzn_oidc claim[%s]=%s does not match %#v", a.Claim, v, m)
	}
	log.Printf("[warn] auth amzn_oidc claim[%s]=%s does not match any matchers", a.Claim, vs)
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
