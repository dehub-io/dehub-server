package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// Storage 存储接口
type Storage interface {
	// Save 保存文件
	Save(path string, data []byte) error

	// Load 加载文件
	Load(path string) ([]byte, error)

	// Exists 检查文件是否存在
	Exists(path string) bool

	// Delete 删除文件
	Delete(path string) error

	// List 列出目录下的文件
	List(dir string) ([]string, error)

	// Reader 获取文件读取器
	Reader(path string) (io.ReadCloser, error)

	// Type 返回存储类型
	Type() string
}

// LocalStorage 本地文件存储
type LocalStorage struct {
	basePath string
}

// NewLocalStorage 创建本地存储
func NewLocalStorage(basePath string) *LocalStorage {
	os.MkdirAll(basePath, 0755)
	return &LocalStorage{basePath: basePath}
}

func (s *LocalStorage) fullPath(path string) string {
	return filepath.Join(s.basePath, path)
}

func (s *LocalStorage) Save(path string, data []byte) error {
	fullPath := s.fullPath(path)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, data, 0644)
}

func (s *LocalStorage) Load(path string) ([]byte, error) {
	return os.ReadFile(s.fullPath(path))
}

func (s *LocalStorage) Exists(path string) bool {
	_, err := os.Stat(s.fullPath(path))
	return err == nil
}

func (s *LocalStorage) Delete(path string) error {
	return os.Remove(s.fullPath(path))
}

func (s *LocalStorage) List(dir string) ([]string, error) {
	entries, err := os.ReadDir(s.fullPath(dir))
	if err != nil {
		return nil, err
	}

	files := []string{}
	for _, entry := range entries {
		files = append(files, entry.Name())
	}
	return files, nil
}

func (s *LocalStorage) Reader(path string) (io.ReadCloser, error) {
	return os.Open(s.fullPath(path))
}

func (s *LocalStorage) Type() string {
	return "local"
}

// RustFSStorage S3 兼容存储 (RustFS)
type RustFSStorage struct {
	endpoint  string
	bucket    string
	accessKey string
	secretKey string
	// TODO: 使用 minio-go 或 aws-sdk-go 实现
}

// NewRustFSStorage 创建 RustFS 存储
func NewRustFSStorage(endpoint, bucket, accessKey, secretKey string) *RustFSStorage {
	return &RustFSStorage{
		endpoint:  endpoint,
		bucket:    bucket,
		accessKey: accessKey,
		secretKey: secretKey,
	}
}

func (s *RustFSStorage) Save(path string, data []byte) error {
	// TODO: 实现 S3 PutObject
	return errors.New("RustFS storage not implemented yet")
}

func (s *RustFSStorage) Load(path string) ([]byte, error) {
	// TODO: 实现 S3 GetObject
	return nil, errors.New("RustFS storage not implemented yet")
}

func (s *RustFSStorage) Exists(path string) bool {
	// TODO: 实现 S3 StatObject
	return false
}

func (s *RustFSStorage) Delete(path string) error {
	// TODO: 实现 S3 RemoveObject
	return errors.New("RustFS storage not implemented yet")
}

func (s *RustFSStorage) List(dir string) ([]string, error) {
	// TODO: 实现 S3 ListObjects
	return nil, errors.New("RustFS storage not implemented yet")
}

func (s *RustFSStorage) Reader(path string) (io.ReadCloser, error) {
	// TODO: 实现 S3 GetObject 返回 ReadCloser
	return nil, errors.New("RustFS storage not implemented yet")
}

func (s *RustFSStorage) Type() string {
	return "rustfs"
}

// NewStorage 根据配置创建存储
func NewStorage(config *StorageConfig) Storage {
	if config == nil {
		return NewLocalStorage("./data")
	}

	switch config.Type {
	case "rustfs":
		return NewRustFSStorage(
			config.Endpoint,
			config.Bucket,
			config.AccessKey,
			config.SecretKey,
		)
	default:
		if config.Path == "" {
			config.Path = "./data"
		}
		return NewLocalStorage(config.Path)
	}
}