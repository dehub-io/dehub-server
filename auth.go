package main

import (
	"net/http"
	"strings"
)

// Auth 认证管理器
type Auth struct {
	config *Config
}

// NewAuth 创建认证管理器
func NewAuth(config *Config) *Auth {
	return &Auth{config: config}
}

// Authenticate 验证 Token，返回用户名
func (a *Auth) Authenticate(r *http.Request) (string, bool) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", false
	}

	// Bearer token
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", false
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	return a.FindUserByToken(token)
}

// FindUserByToken 通过 Token 查找用户
func (a *Auth) FindUserByToken(token string) (string, bool) {
	a.config.mu.RLock()
	defer a.config.mu.RUnlock()

	for username, user := range a.config.Users {
		if user.Token == token {
			return username, true
		}
	}
	return "", false
}

// IsAdmin 检查用户是否是管理员（任意命名空间的 owner）
func (a *Auth) IsAdmin(username string) bool {
	a.config.mu.RLock()
	defer a.config.mu.RUnlock()

	for _, ns := range a.config.Namespaces {
		for _, owner := range ns.Owners {
			if owner == username {
				return true
			}
		}
	}
	return false
}
