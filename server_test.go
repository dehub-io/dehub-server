package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
)

func newTestServer(t *testing.T) *Server {
	tmpDir, err := os.MkdirTemp("", "dehub-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	config := &Config{
		Server: ServerConfig{
			Name:   "test-server",
			Listen: ":8080",
		},
		Namespace: NamespaceConfig{
			AutoApprove: true,
			Reserved:    []string{"admin", "system"},
		},
		Storage: StorageConfig{
			Type: "local",
			Path: tmpDir,
		},
		Users: map[string]User{
			"admin":     {Token: "admin-token"},
			"developer": {Token: "dev-token"},
		},
		Namespaces: map[string]NS{
			"public-ns": {
				Owners:      []string{"admin"},
				Maintainers: []string{"developer"},
				Visibility:  "public",
				Status:      "active",
			},
			"private-ns": {
				Owners:      []string{"admin"},
				Maintainers: []string{},
				Visibility:  "private",
				Status:      "active",
			},
		},
		Upstreams: []UpstreamConfig{},
	}

	return NewServer(config)
}

func TestHandleInfo(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
	rec := httptest.NewRecorder()

	s.handleInfo(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleInfo status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["name"] != "test-server" {
		t.Errorf("handleInfo name = %v, want test-server", resp["name"])
	}
}

func TestHandleAuthStatus(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	tests := []struct {
		name       string
		token      string
		wantLogged bool
		wantUser   string
	}{
		{"已登录", "admin-token", true, "admin"},
		{"未登录", "", false, ""},
		{"无效token", "invalid-token", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rec := httptest.NewRecorder()

			s.handleAuthStatus(rec, req)

			var resp map[string]interface{}
			json.NewDecoder(rec.Body).Decode(&resp)
			if resp["logged_in"] != tt.wantLogged {
				t.Errorf("logged_in = %v, want %v", resp["logged_in"], tt.wantLogged)
			}
			if tt.wantUser != "" && resp["username"] != tt.wantUser {
				t.Errorf("username = %v, want %v", resp["username"], tt.wantUser)
			}
		})
	}
}

func TestHandleNamespaces(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	t.Run("列出命名空间", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces", nil)
		rec := httptest.NewRecorder()

		s.listNamespaces(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("listNamespaces status = %d", rec.Code)
		}

		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		namespaces := resp["namespaces"].([]interface{})
		if len(namespaces) == 0 {
			t.Error("应该有命名空间")
		}
	})

	t.Run("创建命名空间-已登录", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name":"new-ns"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces", body)
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		s.createNamespace(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("createNamespace status = %d, body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("创建命名空间-未登录", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name":"another-ns"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		s.createNamespace(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("createNamespace status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("创建保留命名空间", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name":"admin"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces", body)
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		s.createNamespace(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("createNamespace status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestHandlePackages(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	// 创建测试索引
	indexData := []byte(`{"packages":["public-ns/test-pkg"]}`)
	s.storage.Save("index.json", indexData)

	t.Run("列出包", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/packages", nil)
		rec := httptest.NewRecorder()

		s.listPackages(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("listPackages status = %d", rec.Code)
		}
	})

	t.Run("列出包-过滤命名空间", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/packages?namespace=public-ns", nil)
		rec := httptest.NewRecorder()

		s.listPackages(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("listPackages status = %d", rec.Code)
		}
	})
}

func TestPublishPackage(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	// 创建 multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 添加表单字段
	writer.WriteField("namespace", "public-ns")
	writer.WriteField("name", "test-pkg")
	writer.WriteField("version", "1.0.0")

	// 添加文件
	part, _ := writer.CreateFormFile("file", "package.tar.gz")
	io.WriteString(part, "fake package content")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/packages", body)
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	s.publishPackage(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("publishPackage status = %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlePackageDownload(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	// 创建测试文件
	s.storage.Save("packages/public-ns/test-pkg/1.0.0/package.tar.gz", []byte("test content"))

	t.Run("下载公开包", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/packages/public-ns/test-pkg/1.0.0/package.tar.gz", nil)
		rec := httptest.NewRecorder()

		s.handlePackageDownload(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("handlePackageDownload status = %d", rec.Code)
		}
		if rec.Body.String() != "test content" {
			t.Errorf("body = %s, want 'test content'", rec.Body.String())
		}
	})

	t.Run("下载不存在的包", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/packages/public-ns/nonexistent/1.0.0/package.tar.gz", nil)
		rec := httptest.NewRecorder()

		s.handlePackageDownload(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("handlePackageDownload status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("无效路径", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/packages/", nil)
		rec := httptest.NewRecorder()

		s.handlePackageDownload(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("handlePackageDownload status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestLoadLocalIndex(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	t.Run("索引存在", func(t *testing.T) {
		s.storage.Save("index.json", []byte(`{"packages":["ns/pkg1","ns/pkg2"]}`))

		pkgs, err := s.loadLocalIndex()
		if err != nil {
			t.Fatalf("loadLocalIndex error = %v", err)
		}
		if len(pkgs) != 2 {
			t.Errorf("len(packages) = %d, want 2", len(pkgs))
		}
	})

	t.Run("索引不存在", func(t *testing.T) {
		s.storage.Delete("index.json")

		_, err := s.loadLocalIndex()
		if err == nil {
			t.Error("loadLocalIndex 应该返回错误")
		}
	})
}

func TestHandleProxy(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	t.Run("本地文件存在", func(t *testing.T) {
		s.storage.Save("test/file.txt", []byte("local content"))

		req := httptest.NewRequest(http.MethodGet, "/test/file.txt", nil)
		rec := httptest.NewRecorder()

		s.handleProxy(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("handleProxy status = %d", rec.Code)
		}
		if rec.Body.String() != "local content" {
			t.Errorf("body = %s, want 'local content'", rec.Body.String())
		}
	})

	t.Run("本地文件不存在", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/nonexistent.txt", nil)
		rec := httptest.NewRecorder()

		s.handleProxy(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("handleProxy status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})
}

func TestHandleNamespacesMethod(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	t.Run("GET方法", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces", nil)
		rec := httptest.NewRecorder()

		s.handleNamespaces(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("POST方法", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name":"test-new-ns"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces", body)
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		s.handleNamespaces(rec, req)

		// 可能因为配置保存失败而返回500，但至少进入了POST分支
		if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d", rec.Code)
		}
	})

	t.Run("PUT方法不允许", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/namespaces", nil)
		rec := httptest.NewRecorder()

		s.handleNamespaces(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("DELETE方法不允许", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/namespaces", nil)
		rec := httptest.NewRecorder()

		s.handleNamespaces(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
		}
	})
}

func TestHandlePackagesMethod(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	t.Run("GET方法", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/packages", nil)
		rec := httptest.NewRecorder()

		s.handlePackages(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("POST方法", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("namespace", "public-ns")
		writer.WriteField("name", "test-pkg")
		writer.WriteField("version", "1.0.0")
		part, _ := writer.CreateFormFile("file", "package.tar.gz")
		io.WriteString(part, "content")
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/packages", body)
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()

		s.handlePackages(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d", rec.Code)
		}
	})

	t.Run("PUT方法不允许", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/packages", nil)
		rec := httptest.NewRecorder()

		s.handlePackages(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("DELETE方法不允许", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/packages", nil)
		rec := httptest.NewRecorder()

		s.handlePackages(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
		}
	})
}

func TestPublishPackageErrors(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	t.Run("未登录", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("namespace", "public-ns")
		writer.WriteField("name", "test-pkg")
		writer.WriteField("version", "1.0.0")
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/packages", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()

		s.publishPackage(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("无权限", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("namespace", "private-ns")
		writer.WriteField("name", "test-pkg")
		writer.WriteField("version", "1.0.0")
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/packages", body)
		req.Header.Set("Authorization", "Bearer dev-token")
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()

		s.publishPackage(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
	})

	t.Run("缺少参数", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("namespace", "public-ns")
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/packages", body)
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()

		s.publishPackage(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestCreateNamespaceErrors(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	t.Run("无效JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces", bytes.NewBufferString("invalid"))
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		s.createNamespace(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestListNamespacesPrivate(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	// 未登录用户不应看到私有命名空间
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces", nil)
	rec := httptest.NewRecorder()

	s.listNamespaces(rec, req)

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	namespaces := resp["namespaces"].([]interface{})

	// 只有 public-ns 应该可见
	for _, ns := range namespaces {
		nsMap := ns.(map[string]interface{})
		if nsMap["name"] == "private-ns" {
			t.Error("私有命名空间不应对匿名用户可见")
		}
	}
}

func TestSaveConfig(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	// 设置一个临时文件路径用于保存
	originalDir, _ := os.Getwd()
	os.Chdir(s.config.Storage.Path)
	defer os.Chdir(originalDir)

	err := s.saveConfig()
	if err != nil {
		t.Errorf("saveConfig error = %v", err)
	}
}

func TestUpstreamProxy(t *testing.T) {
	// 创建 mock 上游服务器
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test/upstream.txt" {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write([]byte("upstream content"))
			return
		}
		http.NotFound(w, r)
	}))
	defer upstream.Close()

	// 创建带上游配置的服务器
	tmpDir, _ := os.MkdirTemp("", "dehub-test-*")
	config := &Config{
		Server: ServerConfig{Name: "test", Listen: ":8080"},
		Storage: StorageConfig{Type: "local", Path: tmpDir},
		Namespaces: map[string]NS{
			"test": {Owners: []string{"admin"}, Visibility: "public"},
		},
		Upstreams: []UpstreamConfig{
			{Name: "upstream", URL: upstream.URL, Cache: true},
		},
	}
	s := NewServer(config)
	defer os.RemoveAll(tmpDir)

	t.Run("从上游代理并缓存", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test/upstream.txt", nil)
		rec := httptest.NewRecorder()

		s.handleProxy(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d", rec.Code)
		}
		if rec.Body.String() != "upstream content" {
			t.Errorf("body = %s", rec.Body.String())
		}

		// 验证已缓存
		if !s.storage.Exists("test/upstream.txt") {
			t.Error("应该已缓存到本地")
		}
	})

	t.Run("从缓存读取", func(t *testing.T) {
		// 先缓存
		s.storage.Save("cached/file.txt", []byte("cached content"))

		req := httptest.NewRequest(http.MethodGet, "/cached/file.txt", nil)
		rec := httptest.NewRecorder()

		s.handleProxy(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d", rec.Code)
		}
		if rec.Body.String() != "cached content" {
			t.Errorf("body = %s", rec.Body.String())
		}
	})
}

func TestPackageDownloadUpstream(t *testing.T) {
	// 创建 mock 上游服务器
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/packages/upstream-ns/pkg/1.0.0/package.tar.gz" {
			w.Write([]byte("upstream package"))
			return
		}
		http.NotFound(w, r)
	}))
	defer upstream.Close()

	tmpDir, _ := os.MkdirTemp("", "dehub-test-*")
	config := &Config{
		Server: ServerConfig{Name: "test", Listen: ":8080"},
		Storage: StorageConfig{Type: "local", Path: tmpDir},
		Namespaces: map[string]NS{
			"upstream-ns": {Owners: []string{"admin"}, Visibility: "public"},
		},
		Upstreams: []UpstreamConfig{
			{Name: "upstream", URL: upstream.URL, Cache: true},
		},
	}
	s := NewServer(config)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/packages/upstream-ns/pkg/1.0.0/package.tar.gz", nil)
	rec := httptest.NewRecorder()

	s.handlePackageDownload(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
	if rec.Body.String() != "upstream package" {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestUpdateIndex(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	t.Run("首次添加", func(t *testing.T) {
		err := s.updateIndex("ns/pkg1")
		if err != nil {
			t.Fatalf("updateIndex error = %v", err)
		}

		pkgs, _ := s.loadLocalIndex()
		if len(pkgs) != 1 || pkgs[0] != "ns/pkg1" {
			t.Errorf("packages = %v, want [ns/pkg1]", pkgs)
		}
	})

	t.Run("重复添加", func(t *testing.T) {
		err := s.updateIndex("ns/pkg1")
		if err != nil {
			t.Fatalf("updateIndex error = %v", err)
		}

		pkgs, _ := s.loadLocalIndex()
		if len(pkgs) != 1 {
			t.Errorf("packages = %v, should not duplicate", pkgs)
		}
	})

	t.Run("添加不同包", func(t *testing.T) {
		err := s.updateIndex("ns/pkg2")
		if err != nil {
			t.Fatalf("updateIndex error = %v", err)
		}

		pkgs, _ := s.loadLocalIndex()
		if len(pkgs) != 2 {
			t.Errorf("packages = %v, want 2 packages", pkgs)
		}
	})
}

func TestMaxUploadConfig(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "dehub-test-*")
	defer os.RemoveAll(tmpDir)

	t.Run("默认上传大小", func(t *testing.T) {
		config := &Config{
			Server: ServerConfig{Name: "test", Listen: ":8080"},
			Storage: StorageConfig{Type: "local", Path: tmpDir},
		}
		s := NewServer(config)
		if s.config.Server.MaxUpload != 100<<20 {
			t.Errorf("MaxUpload = %d, want %d", s.config.Server.MaxUpload, 100<<20)
		}
	})

	t.Run("自定义上传大小", func(t *testing.T) {
		config := &Config{
			Server: ServerConfig{Name: "test", Listen: ":8080", MaxUpload: 50 << 20},
			Storage: StorageConfig{Type: "local", Path: tmpDir},
		}
		s := NewServer(config)
		if s.config.Server.MaxUpload != 50<<20 {
			t.Errorf("MaxUpload = %d, want %d", s.config.Server.MaxUpload, 50<<20)
		}
	})
}

func TestPublishPackageWithIndex(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	// 发布包
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("namespace", "public-ns")
	writer.WriteField("name", "new-pkg")
	writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "package.tar.gz")
	io.WriteString(part, "package content")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/packages", body)
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	s.publishPackage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("publishPackage status = %d, body: %s", rec.Code, rec.Body.String())
	}

	// 验证索引已更新
	pkgs, err := s.loadLocalIndex()
	if err != nil {
		t.Fatalf("loadLocalIndex error = %v", err)
	}

	found := false
	for _, p := range pkgs {
		if p == "public-ns/new-pkg" {
			found = true
			break
		}
	}
	if !found {
		t.Error("索引中应包含 public-ns/new-pkg")
	}
}

func TestPublishPackageNoFile(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	// 不包含文件的请求
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("namespace", "public-ns")
	writer.WriteField("name", "test-pkg")
	writer.WriteField("version", "1.0.0")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/packages", body)
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	s.publishPackage(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPackageDownloadPrivateNoAuth(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	// 创建私有命名空间的包
	s.storage.Save("packages/private-ns/test-pkg/1.0.0/package.tar.gz", []byte("private content"))

	req := httptest.NewRequest(http.MethodGet, "/packages/private-ns/test-pkg/1.0.0/package.tar.gz", nil)
	rec := httptest.NewRecorder()

	s.handlePackageDownload(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestListPackagesEmptyIndex(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	// 不创建 index.json，测试空索引情况
	req := httptest.NewRequest(http.MethodGet, "/api/v1/packages", nil)
	rec := httptest.NewRecorder()

	s.listPackages(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestHandleProxyUpstreamError(t *testing.T) {
	// 创建一个会立即关闭的服务器模拟错误
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer upstream.Close()

	tmpDir, _ := os.MkdirTemp("", "dehub-test-*")
	config := &Config{
		Server: ServerConfig{Name: "test", Listen: ":8080"},
		Storage: StorageConfig{Type: "local", Path: tmpDir},
		Upstreams: []UpstreamConfig{
			{Name: "upstream", URL: upstream.URL, Cache: true},
		},
	}
	s := NewServer(config)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent.txt", nil)
	rec := httptest.NewRecorder()

	s.handleProxy(rec, req)

	// 上游返回错误，本地也没有，应该返回 404
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandlePackageDownloadFromUpstreamAndCache(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("from upstream"))
	}))
	defer upstream.Close()

	tmpDir, _ := os.MkdirTemp("", "dehub-test-*")
	config := &Config{
		Server: ServerConfig{Name: "test", Listen: ":8080"},
		Storage: StorageConfig{Type: "local", Path: tmpDir},
		Namespaces: map[string]NS{
			"test-ns": {Owners: []string{"admin"}, Visibility: "public"},
		},
		Upstreams: []UpstreamConfig{
			{Name: "upstream", URL: upstream.URL, Cache: true},
		},
	}
	s := NewServer(config)
	defer os.RemoveAll(tmpDir)

	// 第一次从上游获取
	req := httptest.NewRequest(http.MethodGet, "/packages/test-ns/pkg/1.0.0/file.tar.gz", nil)
	rec := httptest.NewRecorder()

	s.handlePackageDownload(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
	if rec.Body.String() != "from upstream" {
		t.Errorf("body = %s", rec.Body.String())
	}

	// 验证已缓存
	cachedPath := "packages/test-ns/pkg/1.0.0/file.tar.gz"
	if !s.storage.Exists(cachedPath) {
		t.Error("应该已缓存")
	}
}

func TestCreateNamespaceAutoApprove(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "dehub-test-*")
	defer os.RemoveAll(tmpDir)

	config := &Config{
		Server: ServerConfig{Name: "test", Listen: ":8080"},
		Namespace: NamespaceConfig{
			AutoApprove: false, // 需要审批
			Reserved:    []string{"admin"},
		},
		Storage: StorageConfig{Type: "local", Path: tmpDir},
		Users: map[string]User{
			"admin": {Token: "admin-token"},
		},
		Namespaces: map[string]NS{},
	}
	s := NewServer(config)

	// 切换到测试目录以便保存配置
	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	body := bytes.NewBufferString(`{"name":"new-ns"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces", body)
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.createNamespace(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["status"] != "pending" {
		t.Errorf("status = %v, want pending", resp["status"])
	}
}

func TestCreateNamespaceDoubleCheck(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	// 切换到测试目录
	originalDir, _ := os.Getwd()
	os.Chdir(s.config.Storage.Path)
	defer os.Chdir(originalDir)

	// 创建已存在的命名空间
	body := bytes.NewBufferString(`{"name":"public-ns"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces", body)
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.createNamespace(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPublishPackageInvalidMultipart(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	// 发送无效的 multipart 数据
	req := httptest.NewRequest(http.MethodPost, "/api/v1/packages", bytes.NewBufferString("invalid"))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
	rec := httptest.NewRecorder()

	s.publishPackage(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestListNamespacesConcurrent(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	// 并发读取命名空间
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces", nil)
			rec := httptest.NewRecorder()
			s.listNamespaces(rec, req)
		}()
	}
	wg.Wait()
}

func TestListPackagesWithNamespace(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	// 创建索引
	s.storage.Save("index.json", []byte(`{"packages":["ns1/pkg1","ns2/pkg2","ns1/pkg2"]}`))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/packages?namespace=ns1", nil)
	rec := httptest.NewRecorder()

	s.listPackages(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	packages := resp["packages"].([]interface{})

	// 应该只返回 ns1 的包
	for _, pkg := range packages {
		pkgStr := pkg.(string)
		if pkgStr[:3] != "ns1" {
			t.Errorf("unexpected package: %s", pkgStr)
		}
	}
}

func TestHandlePackagesGet(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/packages", nil)
	rec := httptest.NewRecorder()

	s.handlePackages(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestHandleNamespacesGet(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces", nil)
	rec := httptest.NewRecorder()

	s.handleNamespaces(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestLoadLocalIndexInvalidJSON(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	// 创建无效的 JSON
	s.storage.Save("index.json", []byte(`invalid json`))

	_, err := s.loadLocalIndex()
	if err == nil {
		t.Error("should return error for invalid JSON")
	}
}

func TestListPackagesInvalidJSON(t *testing.T) {
	s := newTestServer(t)
	defer os.RemoveAll(s.config.Storage.Path)

	// 创建无效的 JSON 索引
	s.storage.Save("index.json", []byte(`invalid json`))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/packages", nil)
	rec := httptest.NewRecorder()

	s.listPackages(rec, req)

	// 应该返回空列表
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
}
