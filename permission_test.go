package main

import (
	"testing"
)

func TestPermissionCanRead(t *testing.T) {
	perm := NewPermission(&Config{
		Namespace: NamespaceConfig{
			Reserved: []string{"admin", "system"},
		},
		Namespaces: map[string]NS{
			"public-ns": {
				Owners:      []string{"admin"},
				Maintainers: []string{"dev"},
				Visibility:  "public",
			},
			"private-ns": {
				Owners:      []string{"admin"},
				Maintainers: []string{"dev"},
				Visibility:  "private",
			},
		},
	})

	tests := []struct {
		name      string
		username  string
		namespace string
		want      bool
	}{
		{"公开命名空间-匿名用户", "", "public-ns", true},
		{"公开命名空间-任意用户", "anyone", "public-ns", true},
		{"私有命名空间-owner", "admin", "private-ns", true},
		{"私有命名空间-maintainer", "dev", "private-ns", true},
		{"私有命名空间-匿名用户", "", "private-ns", false},
		{"私有命名空间-无权限用户", "anyone", "private-ns", false},
		{"不存在的命名空间", "admin", "nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := perm.CanRead(tt.username, tt.namespace); got != tt.want {
				t.Errorf("CanRead(%s, %s) = %v, want %v", tt.username, tt.namespace, got, tt.want)
			}
		})
	}
}

func TestPermissionCanPublish(t *testing.T) {
	perm := NewPermission(&Config{
		Namespaces: map[string]NS{
			"test-ns": {
				Owners:      []string{"admin"},
				Maintainers: []string{"dev"},
				Visibility:  "public",
			},
		},
	})

	tests := []struct {
		name      string
		username  string
		namespace string
		want      bool
	}{
		{"owner 可以发布", "admin", "test-ns", true},
		{"maintainer 可以发布", "dev", "test-ns", true},
		{"无权限用户不能发布", "anyone", "test-ns", false},
		{"匿名用户不能发布", "", "test-ns", false},
		{"不存在的命名空间", "admin", "nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := perm.CanPublish(tt.username, tt.namespace); got != tt.want {
				t.Errorf("CanPublish(%s, %s) = %v, want %v", tt.username, tt.namespace, got, tt.want)
			}
		})
	}
}

func TestPermissionCanManageNamespace(t *testing.T) {
	perm := NewPermission(&Config{
		Namespaces: map[string]NS{
			"test-ns": {
				Owners:      []string{"admin"},
				Maintainers: []string{"dev"},
			},
		},
	})

	tests := []struct {
		name      string
		username  string
		namespace string
		want      bool
	}{
		{"owner 可以管理", "admin", "test-ns", true},
		{"maintainer 不能管理", "dev", "test-ns", false},
		{"无权限用户不能管理", "anyone", "test-ns", false},
		{"不存在的命名空间", "admin", "nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := perm.CanManageNamespace(tt.username, tt.namespace); got != tt.want {
				t.Errorf("CanManageNamespace(%s, %s) = %v, want %v", tt.username, tt.namespace, got, tt.want)
			}
		})
	}
}

func TestPermissionCanCreateNamespace(t *testing.T) {
	perm := NewPermission(&Config{
		Namespace: NamespaceConfig{
			Reserved:    []string{"admin", "system", "dehub"},
			AutoApprove: true,
		},
		Namespaces: map[string]NS{
			"existing-ns": {Owners: []string{"admin"}},
		},
	})

	tests := []struct {
		name      string
		username  string
		namespace string
		wantOK    bool
		wantReason string
	}{
		{"可以创建", "dev", "new-ns", true, ""},
		{"保留命名空间", "dev", "admin", false, "命名空间被保留"},
		{"已存在的命名空间", "dev", "existing-ns", false, "命名空间已存在"},
		{"保留命名空间大小写不敏感", "dev", "ADMIN", false, "命名空间被保留"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOK, gotReason := perm.CanCreateNamespace(tt.username, tt.namespace)
			if gotOK != tt.wantOK {
				t.Errorf("CanCreateNamespace() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotReason != tt.wantReason {
				t.Errorf("CanCreateNamespace() reason = %s, want %s", gotReason, tt.wantReason)
			}
		})
	}
}

func TestPermissionIsOwnerOrMaintainer(t *testing.T) {
	perm := NewPermission(&Config{
		Namespaces: map[string]NS{
			"test-ns": {
				Owners:      []string{"owner1"},
				Maintainers: []string{"maintainer1"},
			},
		},
	})

	tests := []struct {
		name      string
		username  string
		namespace string
		want      bool
	}{
		{"是 owner", "owner1", "test-ns", true},
		{"是 maintainer", "maintainer1", "test-ns", true},
		{"都不是", "unknown", "test-ns", false},
		{"不存在的命名空间", "owner1", "nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := perm.isOwnerOrMaintainer(tt.username, tt.namespace); got != tt.want {
				t.Errorf("isOwnerOrMaintainer(%s, %s) = %v, want %v", tt.username, tt.namespace, got, tt.want)
			}
		})
	}
}
