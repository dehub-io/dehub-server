package main

import (
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config 服务端配置
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Namespace  NamespaceConfig  `yaml:"namespace"`
	Storage    StorageConfig    `yaml:"storage"`
	Users      map[string]User  `yaml:"users"`
	Namespaces map[string]NS    `yaml:"namespaces"`
	Upstreams  []UpstreamConfig `yaml:"upstreams"`

	// 运行时状态（不序列化）
	mu sync.RWMutex `yaml:"-"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Name       string `yaml:"name"`
	Listen     string `yaml:"listen"`
	MaxUpload  int64  `yaml:"max_upload"` // 最大上传大小（字节），默认 100MB
}

// NamespaceConfig 命名空间配置
type NamespaceConfig struct {
	AutoApprove bool     `yaml:"auto_approve"`
	Reserved    []string `yaml:"reserved"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type      string `yaml:"type"`       // local 或 rustfs
	Path      string `yaml:"path"`       // 本地存储路径
	Endpoint  string `yaml:"endpoint"`   // RustFS endpoint
	Bucket    string `yaml:"bucket"`     // RustFS bucket
	AccessKey string `yaml:"access_key"` // RustFS access key
	SecretKey string `yaml:"secret_key"` // RustFS secret key
}

// User 用户配置
type User struct {
	Token string `yaml:"token"`
}

// NS 命名空间配置
type NS struct {
	Owners      []string `yaml:"owners"`
	Maintainers []string `yaml:"maintainers"`
	Visibility  string   `yaml:"visibility"` // public 或 private
	Status      string   `yaml:"status"`     // active 或 pending
}

// UpstreamConfig 上游仓库配置
type UpstreamConfig struct {
	Name  string `yaml:"name"`
	URL   string `yaml:"url"`
	Cache bool   `yaml:"cache"`
}

// LoadConfig 加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := &Config{
		Server: ServerConfig{
			Name:   "dehub-server",
			Listen: ":8080",
		},
		Namespace: NamespaceConfig{
			AutoApprove: true,
			Reserved:    []string{"admin", "system", "dehub"},
		},
		Storage: StorageConfig{
			Type: "local",
			Path: "./data",
		},
		Users:      make(map[string]User),
		Namespaces: make(map[string]NS),
		Upstreams:  []UpstreamConfig{},
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}