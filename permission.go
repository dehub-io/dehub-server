package main

import "strings"

// Permission 权限管理器
type Permission struct {
	config *Config
}

// NewPermission 创建权限管理器
func NewPermission(config *Config) *Permission {
	return &Permission{config: config}
}

// CanRead 检查用户是否可以读取包
func (p *Permission) CanRead(username, namespace string) bool {
	p.config.mu.RLock()
	defer p.config.mu.RUnlock()

	ns, exists := p.config.Namespaces[namespace]
	if !exists {
		return false
	}

	// 公开包，任何人可读
	if ns.Visibility == "public" {
		return true
	}

	// 私有包，需要是 owner 或 maintainer
	return p.isOwnerOrMaintainerLocked(username, namespace)
}

// CanPublish 检查用户是否可以发布包
func (p *Permission) CanPublish(username, namespace string) bool {
	p.config.mu.RLock()
	defer p.config.mu.RUnlock()
	return p.isOwnerOrMaintainerLocked(username, namespace)
}

// CanManageNamespace 检查用户是否可以管理命名空间
func (p *Permission) CanManageNamespace(username, namespace string) bool {
	p.config.mu.RLock()
	defer p.config.mu.RUnlock()

	ns, exists := p.config.Namespaces[namespace]
	if !exists {
		return false
	}

	for _, owner := range ns.Owners {
		if owner == username {
			return true
		}
	}
	return false
}

// CanCreateNamespace 检查用户是否可以创建命名空间
func (p *Permission) CanCreateNamespace(username, namespace string) (bool, string) {
	p.config.mu.RLock()
	defer p.config.mu.RUnlock()

	// 检查保留命名空间
	for _, reserved := range p.config.Namespace.Reserved {
		if strings.EqualFold(namespace, reserved) {
			return false, "命名空间被保留"
		}
	}

	// 检查是否已存在
	if _, exists := p.config.Namespaces[namespace]; exists {
		return false, "命名空间已存在"
	}

	return true, ""
}

// isOwnerOrMaintainerLocked 检查用户是否是 owner 或 maintainer（需要已持有锁）
func (p *Permission) isOwnerOrMaintainerLocked(username, namespace string) bool {
	ns, exists := p.config.Namespaces[namespace]
	if !exists {
		return false
	}

	for _, owner := range ns.Owners {
		if owner == username {
			return true
		}
	}

	for _, maintainer := range ns.Maintainers {
		if maintainer == username {
			return true
		}
	}

	return false
}

// isOwnerOrMaintainer 检查用户是否是 owner 或 maintainer
func (p *Permission) isOwnerOrMaintainer(username, namespace string) bool {
	p.config.mu.RLock()
	defer p.config.mu.RUnlock()
	return p.isOwnerOrMaintainerLocked(username, namespace)
}
