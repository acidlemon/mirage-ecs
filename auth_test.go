package mirageecs

import (
	"net/http"
	"testing"
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
			b := &AuthMethodToken{
				Token:  tt.fields.Token,
				Header: tt.fields.Header,
			}
			if got := b.Match(tt.args.h); got != tt.want {
				t.Errorf("AuthMethodToken.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthMethodAmznOIDC_Match(t *testing.T) {
	type fields struct {
		Claim    string
		Matchers []*ClaimMatcher
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
				Matchers: []*ClaimMatcher{
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
				Matchers: []*ClaimMatcher{
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
				Matchers: []*ClaimMatcher{
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
				Matchers: []*ClaimMatcher{
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
				Matchers: []*ClaimMatcher{
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
				Matchers: []*ClaimMatcher{
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
				Matchers: []*ClaimMatcher{
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
				Matchers: []*ClaimMatcher{
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
				Matchers: []*ClaimMatcher{
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
			a := &AuthMethodAmznOIDC{
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
			b := &AuthMethodBasic{
				Username: tt.fields.Username,
				Password: tt.fields.Password,
			}
			if got := b.Match(tt.args.h); got != tt.want {
				t.Errorf("AuthMethodBasic.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}
