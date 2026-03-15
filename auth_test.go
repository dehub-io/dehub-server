package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestConfig() *Config {
	return &Config{
		Users: map[string]User{
			"admin":  {Token: "admin-token"},
			"developer": {Token: "dev-token"},
		},
		Namespaces: map[string]NS{
			"test-ns": {
				Owners:      []string{"admin"},
				Maintainers: []string{"developer"},
				Visibility:  "public",
				Status:      "active",
			},
		},
	}
}

func TestAuthenticate(t *testing.T) {
	auth := NewAuth(newTestConfig())

	tests := []struct {
		name       string
		authHeader string
		wantUser   string
		wantOK     bool
	}{
		{
			name:       "有效 admin token",
			authHeader: "Bearer admin-token",
			wantUser:   "admin",
			wantOK:     true,
		},
		{
			name:       "有效 developer token",
			authHeader: "Bearer dev-token",
			wantUser:   "developer",
			wantOK:     true,
		},
		{
			name:       "无效 token",
			authHeader: "Bearer invalid-token",
			wantUser:   "",
			wantOK:     false,
		},
		{
			name:       "无 Bearer 前缀",
			authHeader: "admin-token",
			wantUser:   "",
			wantOK:     false,
		},
		{
			name:       "空 Authorization",
			authHeader: "",
			wantUser:   "",
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			gotUser, gotOK := auth.Authenticate(req)
			if gotUser != tt.wantUser {
				t.Errorf("Authenticate() user = %s, want %s", gotUser, tt.wantUser)
			}
			if gotOK != tt.wantOK {
				t.Errorf("Authenticate() ok = %v, want %v", gotOK, tt.wantOK)
			}
		})
	}
}

func TestFindUserByToken(t *testing.T) {
	auth := NewAuth(newTestConfig())

	tests := []struct {
		token    string
		wantUser string
		wantOK   bool
	}{
		{"admin-token", "admin", true},
		{"dev-token", "developer", true},
		{"unknown-token", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			gotUser, gotOK := auth.FindUserByToken(tt.token)
			if gotUser != tt.wantUser {
				t.Errorf("FindUserByToken() user = %s, want %s", gotUser, tt.wantUser)
			}
			if gotOK != tt.wantOK {
				t.Errorf("FindUserByToken() ok = %v, want %v", gotOK, tt.wantOK)
			}
		})
	}
}

func TestIsAdmin(t *testing.T) {
	auth := NewAuth(newTestConfig())

	tests := []struct {
		username string
		want     bool
	}{
		{"admin", true},      // 是 owner
		{"developer", false}, // 只是 maintainer
		{"unknown", false},   // 无权限
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			if got := auth.IsAdmin(tt.username); got != tt.want {
				t.Errorf("IsAdmin(%s) = %v, want %v", tt.username, got, tt.want)
			}
		})
	}
}
