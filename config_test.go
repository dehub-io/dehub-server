package main

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// 创建临时配置文件
	content := `
server:
  name: "test-server"
  listen: ":9090"

storage:
  type: local
  path: "./test-data"

users:
  admin:
    token: "test-token-123"

namespaces:
  test-ns:
    owners: [admin]
    maintainers: []
    visibility: public
    status: active
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(content)
	tmpFile.Close()

	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadConfig 失败: %v", err)
	}

	// 测试服务器配置
	if config.Server.Name != "test-server" {
		t.Errorf("Server.Name = %s, want test-server", config.Server.Name)
	}
	if config.Server.Listen != ":9090" {
		t.Errorf("Server.Listen = %s, want :9090", config.Server.Listen)
	}

	// 测试存储配置
	if config.Storage.Type != "local" {
		t.Errorf("Storage.Type = %s, want local", config.Storage.Type)
	}

	// 测试用户配置
	if user, ok := config.Users["admin"]; !ok {
		t.Error("Users[admin] not found")
	} else if user.Token != "test-token-123" {
		t.Errorf("Users[admin].Token = %s, want test-token-123", user.Token)
	}

	// 测试命名空间配置
	if ns, ok := config.Namespaces["test-ns"]; !ok {
		t.Error("Namespaces[test-ns] not found")
	} else if ns.Visibility != "public" {
		t.Errorf("Namespaces[test-ns].Visibility = %s, want public", ns.Visibility)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	// 空配置文件
	content := ``
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(content)
	tmpFile.Close()

	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadConfig 失败: %v", err)
	}

	// 测试默认值
	if config.Server.Name != "dehub-server" {
		t.Errorf("默认 Server.Name = %s, want dehub-server", config.Server.Name)
	}
	if config.Server.Listen != ":8080" {
		t.Errorf("默认 Server.Listen = %s, want :8080", config.Server.Listen)
	}
	if config.Storage.Type != "local" {
		t.Errorf("默认 Storage.Type = %s, want local", config.Storage.Type)
	}
	if config.Storage.Path != "./data" {
		t.Errorf("默认 Storage.Path = %s, want ./data", config.Storage.Path)
	}
}

func TestLoadConfigFileNotExist(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Error("LoadConfig 应该返回错误")
	}
}
