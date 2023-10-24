package mirageecs_test

import (
	"net/http"
	"testing"
	"time"

	mirageecs "github.com/acidlemon/mirage-ecs"
)

func TestAuthMethodToken_Match(t *testing.T) {
	type fields struct {
		Token  string
		Header string
	}
	type args struct {
		h http.Header
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "Token matches",
			fields: fields{
				Token:  "mytoken",
				Header: "Authorization",
			},
			args: args{
				h: http.Header{
					"Authorization": []string{"mytoken"},
				},
			},
			want: true,
		},
		{
			name: "Token does not match",
			fields: fields{
				Token:  "mytoken",
				Header: "Authorization",
			},
			args: args{
				h: http.Header{
					"Authorization": []string{"othertoken"},
				},
			},
			want: false,
		},
		{
			name: "Token is empty",
			fields: fields{
				Token:  "",
				Header: "Authorization",
			},
			args: args{
				h: http.Header{
					"Authorization": []string{"mytoken"},
				},
			},
			want: false,
		},
		{
			name: "Header is empty",
			fields: fields{
				Token:  "mytoken",
				Header: "",
			},
			args: args{
				h: http.Header{
					"Authorization": []string{"mytoken"},
				},
			},
			want: false,
		},
		{
			name: "Header does not exist",
			fields: fields{
				Token:  "mytoken",
				Header: "Authorization",
			},
			args: args{
				h: http.Header{},
			},
			want: false,
		},
		{
			name: "Nil AuthMethodToken",
			fields: fields{
				Token:  "",
				Header: "",
			},
			args: args{
				h: http.Header{},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &mirageecs.AuthMethodToken{
				Token:  tt.fields.Token,
				Header: tt.fields.Header,
			}
			if got := b.Match(tt.args.h); got != tt.want {
				t.Errorf("mirageecs.mirageecs.AuthMethodToken.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthMethodAmznOIDC_Match(t *testing.T) {
	type fields struct {
		Claim    string
		Matchers []*mirageecs.ClaimMatcher
	}
	type args struct {
		Claims map[string]any
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "Claim matches",
			fields: fields{
				Claim: "email",
				Matchers: []*mirageecs.ClaimMatcher{
					{
						Exact: "user@example.com",
					},
				},
			},
			args: args{
				Claims: map[string]any{
					"email": "user@example.com",
				},
			},
			want: true,
		},
		{
			name: "Claim does not match",
			fields: fields{
				Claim: "email",
				Matchers: []*mirageecs.ClaimMatcher{
					{
						Exact: "user@example.com",
					},
				},
			},
			args: args{
				Claims: map[string]any{
					"email": "xxx@example.com",
				},
			},
			want: false,
		},
		{
			name: "Claim is empty",
			fields: fields{
				Claim: "",
				Matchers: []*mirageecs.ClaimMatcher{
					{
						Exact: "user@example.com",
					},
				},
			},
			args: args{
				Claims: map[string]any{
					"email": "user@example.com",
				},
			},
			want: false,
		},
		{
			name: "Claim matches suffix",
			fields: fields{
				Claim: "email",
				Matchers: []*mirageecs.ClaimMatcher{
					{
						Suffix: "@example.com",
					},
				},
			},
			args: args{
				Claims: map[string]any{
					"email": "user@example.com",
				},
			},
			want: true,
		},
		{
			name: "Claim does not match suffix",
			fields: fields{
				Claim: "email",
				Matchers: []*mirageecs.ClaimMatcher{
					{
						Suffix: "@example.com",
					},
				},
			},
			args: args{
				Claims: map[string]any{
					"email": "user@example.net",
				},
			},
			want: false,
		},
		{
			name: "Claim matches any suffix",
			fields: fields{
				Claim: "email",
				Matchers: []*mirageecs.ClaimMatcher{
					{
						Suffix: "@example.com",
					},
					{
						Suffix: "@example.net",
					},
				},
			},
			args: args{
				Claims: map[string]any{
					"email": "user@example.net",
				},
			},
			want: true,
		},
		{
			name: "Claim matches any exact",
			fields: fields{
				Claim: "email",
				Matchers: []*mirageecs.ClaimMatcher{
					{
						Exact: "foo@example.com",
					},
					{
						Exact: "bar@example.net",
					},
				},
			},
			args: args{
				Claims: map[string]any{
					"email": "bar@example.net",
				},
			},
			want: true,
		},
		{
			name: "Claim match both",
			fields: fields{
				Claim: "email",
				Matchers: []*mirageecs.ClaimMatcher{
					{
						Suffix: "@example.com",
					},
					{
						Exact: "user@example.com",
					},
				},
			},
			args: args{
				Claims: map[string]any{
					"email": "user@example.com",
				},
			},
			want: true,
		},
		{
			name: "Claim does not match both",
			fields: fields{
				Claim: "email",
				Matchers: []*mirageecs.ClaimMatcher{
					{
						Suffix: "@example.net",
					},
					{
						Exact: "user@example.net",
					},
				},
			},
			args: args{
				Claims: map[string]any{
					"email": "user@example.com",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &mirageecs.AuthMethodAmznOIDC{
				Claim:    tt.fields.Claim,
				Matchers: tt.fields.Matchers,
			}
			got := a.MatchClaims(tt.args.Claims)
			if got != tt.want {
				t.Errorf("AuthMethodAmznOIDC.MatchClaims() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthMethodBasic_Match(t *testing.T) {
	type fields struct {
		Username string
		Password string
	}
	type args struct {
		h http.Header
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "Username and password match",
			fields: fields{
				Username: "user",
				Password: "pass",
			},
			args: args{
				h: http.Header{
					"Authorization": []string{"Basic dXNlcjpwYXNz"},
				},
			},
			want: true,
		},
		{
			name: "Username and password do not match",
			fields: fields{
				Username: "user",
				Password: "pass",
			},
			args: args{
				h: http.Header{
					"Authorization": []string{"Basic dXNlcnM6cGFzcw=="},
				},
			},
			want: false,
		},
		{
			name: "Username is empty",
			fields: fields{
				Username: "",
				Password: "pass",
			},
			args: args{
				h: http.Header{
					"Authorization": []string{"Basic OnBhc3M="},
				},
			},
			want: false,
		},
		{
			name: "Password is empty",
			fields: fields{
				Username: "user",
				Password: "",
			},
			args: args{
				h: http.Header{
					"Authorization": []string{"Basic dXNlcjo="},
				},
			},
			want: false,
		},
		{
			name: "Authorization header is empty",
			fields: fields{
				Username: "user",
				Password: "pass",
			},
			args: args{
				h: http.Header{},
			},
			want: false,
		},
		{
			name: "Nil AuthMethodBasic",
			fields: fields{
				Username: "",
				Password: "",
			},
			args: args{
				h: http.Header{},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &mirageecs.AuthMethodBasic{
				Username: tt.fields.Username,
				Password: tt.fields.Password,
			}
			if got := b.Match(tt.args.h); got != tt.want {
				t.Errorf("AuthMethodBasic.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthCookie(t *testing.T) {
	auth := mirageecs.Auth{
		CookieSecret: "secret",
	}
	cookie, err := auth.NewAuthCookie(time.Second, ".example.com")
	if err != nil {
		t.Error(err)
	}
	if cookie.Name != "mirage-ecs-auth" {
		t.Errorf("invalid cookie name: %s", cookie.Name)
	}
	if cookie.Value == "" {
		t.Errorf("invalid cookie value: %s", cookie.Value)
	}
	if cookie.Domain != ".example.com" {
		t.Errorf("invalid cookie domain: %s", cookie.Domain)
	}
	if cookie.HttpOnly != true {
		t.Errorf("invalid cookie httponly: %v", cookie.HttpOnly)
	}
	if cookie.Expires.IsZero() {
		t.Errorf("invalid cookie expires: %v", cookie.Expires)
	}
	if err := auth.ValidateAuthCookie(cookie); err != nil {
		t.Error(err)
	}

	// invalid cookie
	orig := cookie.Value
	cookie.Value = cookie.Value + "xxx"
	if err := auth.ValidateAuthCookie(cookie); err == nil {
		t.Error("should be invalid")
	}
	// restore
	cookie.Value = orig
	t.Log(cookie.Value)

	// expired
	time.Sleep(2 * time.Second)
	if err := auth.ValidateAuthCookie(cookie); err == nil {
		t.Error("should be expired")
	}
}
