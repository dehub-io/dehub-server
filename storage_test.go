package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorage(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "storage-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage := NewLocalStorage(tmpDir)

	t.Run("Type", func(t *testing.T) {
		if storage.Type() != "local" {
			t.Errorf("Type() = %s, want local", storage.Type())
		}
	})

	t.Run("Save and Load", func(t *testing.T) {
		data := []byte("hello world")
		err := storage.Save("test/file.txt", data)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		got, err := storage.Load("test/file.txt")
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if string(got) != string(data) {
			t.Errorf("Load() = %s, want %s", got, data)
		}
	})

	t.Run("Exists", func(t *testing.T) {
		if !storage.Exists("test/file.txt") {
			t.Error("Exists() = false, want true")
		}
		if storage.Exists("nonexistent.txt") {
			t.Error("Exists(nonexistent) = true, want false")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		storage.Save("to-delete.txt", []byte("data"))
		if !storage.Exists("to-delete.txt") {
			t.Fatal("文件应该存在")
		}

		err := storage.Delete("to-delete.txt")
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		if storage.Exists("to-delete.txt") {
			t.Error("Delete() 后文件仍存在")
		}
	})

	t.Run("List", func(t *testing.T) {
		storage.Save("list/a.txt", []byte("a"))
		storage.Save("list/b.txt", []byte("b"))

		files, err := storage.List("list")
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if len(files) != 2 {
			t.Errorf("List() 返回 %d 文件, want 2", len(files))
		}
	})

	t.Run("Reader", func(t *testing.T) {
		data := []byte("reader test")
		storage.Save("reader-test.txt", data)

		reader, err := storage.Reader("reader-test.txt")
		if err != nil {
			t.Fatalf("Reader() error = %v", err)
		}
		defer reader.Close()

		got, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}

		if string(got) != string(data) {
			t.Errorf("Reader content = %s, want %s", got, data)
		}
	})

	t.Run("自动创建目录", func(t *testing.T) {
		err := storage.Save("deeply/nested/path/file.txt", []byte("nested"))
		if err != nil {
			t.Fatalf("Save() 深层目录 error = %v", err)
		}

		// 验证目录结构
		fullPath := filepath.Join(tmpDir, "deeply/nested/path/file.txt")
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Error("深层目录文件未创建")
		}
	})
}

func TestNewStorage(t *testing.T) {
	t.Run("nil config 默认本地存储", func(t *testing.T) {
		storage := NewStorage(nil)
		if storage.Type() != "local" {
			t.Errorf("Type() = %s, want local", storage.Type())
		}
	})

	t.Run("local 类型", func(t *testing.T) {
		storage := NewStorage(&StorageConfig{
			Type: "local",
			Path: "./test-data",
		})
		if storage.Type() != "local" {
			t.Errorf("Type() = %s, want local", storage.Type())
		}
	})

	t.Run("rustfs 类型", func(t *testing.T) {
		storage := NewStorage(&StorageConfig{
			Type:      "rustfs",
			Endpoint:  "http://localhost:9000",
			Bucket:    "test",
			AccessKey: "key",
			SecretKey: "secret",
		})
		if storage.Type() != "rustfs" {
			t.Errorf("Type() = %s, want rustfs", storage.Type())
		}
	})

	t.Run("空路径使用默认值", func(t *testing.T) {
		config := &StorageConfig{Type: "local", Path: ""}
		storage := NewStorage(config)
		if config.Path != "./data" {
			t.Errorf("Path = %s, want ./data", config.Path)
		}
		if storage.Type() != "local" {
			t.Errorf("Type() = %s, want local", storage.Type())
		}
	})
}

func TestRustFSStorageNotImplemented(t *testing.T) {
	storage := NewRustFSStorage("http://localhost:9000", "test", "key", "secret")

	t.Run("Type", func(t *testing.T) {
		if storage.Type() != "rustfs" {
			t.Errorf("Type() = %s, want rustfs", storage.Type())
		}
	})

	t.Run("Save - 未实现", func(t *testing.T) {
		err := storage.Save("test.txt", []byte("data"))
		if err == nil {
			t.Error("Save 应该返回错误")
		}
	})

	t.Run("Load - 未实现", func(t *testing.T) {
		_, err := storage.Load("test.txt")
		if err == nil {
			t.Error("Load 应该返回错误")
		}
	})

	t.Run("Exists - 未实现", func(t *testing.T) {
		if storage.Exists("test.txt") {
			t.Error("Exists 应该返回 false")
		}
	})

	t.Run("Delete - 未实现", func(t *testing.T) {
		err := storage.Delete("test.txt")
		if err == nil {
			t.Error("Delete 应该返回错误")
		}
	})

	t.Run("List - 未实现", func(t *testing.T) {
		_, err := storage.List(".")
		if err == nil {
			t.Error("List 应该返回错误")
		}
	})

	t.Run("Reader - 未实现", func(t *testing.T) {
		_, err := storage.Reader("test.txt")
		if err == nil {
			t.Error("Reader 应该返回错误")
		}
	})
}

func TestLocalStorageErrors(t *testing.T) {
	t.Run("Load不存在文件", func(t *testing.T) {
		tmpDir, _ := os.MkdirTemp("", "storage-test-*")
		defer os.RemoveAll(tmpDir)
		storage := NewLocalStorage(tmpDir)

		_, err := storage.Load("nonexistent.txt")
		if err == nil {
			t.Error("Load 不存在的文件应该返回错误")
		}
	})

	t.Run("Delete不存在文件", func(t *testing.T) {
		tmpDir, _ := os.MkdirTemp("", "storage-test-*")
		defer os.RemoveAll(tmpDir)
		storage := NewLocalStorage(tmpDir)

		err := storage.Delete("nonexistent.txt")
		if err == nil {
			t.Error("Delete 不存在的文件应该返回错误")
		}
	})

	t.Run("List不存在目录", func(t *testing.T) {
		tmpDir, _ := os.MkdirTemp("", "storage-test-*")
		defer os.RemoveAll(tmpDir)
		storage := NewLocalStorage(tmpDir)

		_, err := storage.List("nonexistent-dir")
		if err == nil {
			t.Error("List 不存在的目录应该返回错误")
		}
	})

	t.Run("Reader不存在文件", func(t *testing.T) {
		tmpDir, _ := os.MkdirTemp("", "storage-test-*")
		defer os.RemoveAll(tmpDir)
		storage := NewLocalStorage(tmpDir)

		_, err := storage.Reader("nonexistent.txt")
		if err == nil {
			t.Error("Reader 不存在的文件应该返回错误")
		}
	})

	t.Run("Save创建目录失败", func(t *testing.T) {
		// 创建一个文件作为basePath，这样MkdirAll会失败
		tmpFile, err := os.CreateTemp("", "storage-test-*")
		if err != nil {
			t.Fatalf("创建临时文件失败: %v", err)
		}
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		storage := NewLocalStorage(tmpFile.Name())
		err = storage.Save("test/file.txt", []byte("data"))
		if err == nil {
			t.Error("Save 在无效路径应该返回错误")
		}
	})
}
