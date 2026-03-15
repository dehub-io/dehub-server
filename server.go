package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Server dehub 服务端
type Server struct {
	config     *Config
	auth       *Auth
	permission *Permission
	storage    Storage
	http       *http.Server
	client     *http.Client
}

// NewServer 创建服务端
func NewServer(config *Config) *Server {
	// 设置默认上传大小
	if config.Server.MaxUpload == 0 {
		config.Server.MaxUpload = 100 << 20 // 100MB
	}

	return &Server{
		config:     config,
		auth:       NewAuth(config),
		permission: NewPermission(config),
		storage:    NewStorage(&config.Storage),
		http: &http.Server{
			Addr: config.Server.Listen,
		},
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Start 启动服务
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API 路由
	mux.HandleFunc("/api/v1/info", s.handleInfo)
	mux.HandleFunc("/api/v1/auth/status", s.handleAuthStatus)
	mux.HandleFunc("/api/v1/namespaces", s.handleNamespaces)
	mux.HandleFunc("/api/v1/packages", s.handlePackages)
	mux.HandleFunc("/packages/", s.handlePackageDownload)

	// 兜底：代理或本地文件
	mux.HandleFunc("/", s.handleProxy)

	s.http.Handler = mux

	log.Printf("dehub-server [%s] 启动于 %s (storage: %s)", s.config.Server.Name, s.config.Server.Listen, s.storage.Type())
	return s.http.ListenAndServe()
}

// Stop 停止服务
func (s *Server) Stop() {
	s.http.Close()
}

// handleInfo 服务信息
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":    s.config.Server.Name,
		"version": "1.0.0",
		"type":    "dehub-server",
	})
}

// handleAuthStatus 认证状态
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	username, ok := s.auth.Authenticate(r)
	if !ok {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"logged_in": false,
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"logged_in": true,
		"username":  username,
	})
}

// handleNamespaces 命名空间管理
func (s *Server) handleNamespaces(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.listNamespaces(w, r)
	case "POST":
		s.createNamespace(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listNamespaces 列出命名空间
func (s *Server) listNamespaces(w http.ResponseWriter, r *http.Request) {
	username, _ := s.auth.Authenticate(r)

	s.config.mu.RLock()
	namespacesCopy := make(map[string]NS, len(s.config.Namespaces))
	for name, ns := range s.config.Namespaces {
		namespacesCopy[name] = ns
	}
	s.config.mu.RUnlock()

	namespaces := []map[string]interface{}{}
	for name, ns := range namespacesCopy {
		// 私有命名空间只对有权限的用户显示
		if ns.Visibility == "private" && username == "" {
			continue
		}
		if ns.Visibility == "private" && !s.permission.isOwnerOrMaintainer(username, name) {
			continue
		}

		namespaces = append(namespaces, map[string]interface{}{
			"name":       name,
			"visibility": ns.Visibility,
			"status":     ns.Status,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"namespaces": namespaces,
	})
}

// createNamespace 创建命名空间
func (s *Server) createNamespace(w http.ResponseWriter, r *http.Request) {
	username, ok := s.auth.Authenticate(r)
	if !ok {
		http.Error(w, "未授权", http.StatusUnauthorized)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效请求", http.StatusBadRequest)
		return
	}

	// 检查并创建命名空间（需要写锁保护整个操作）
	s.config.mu.Lock()
	defer s.config.mu.Unlock()

	// 再次检查（双检锁模式）
	if _, exists := s.config.Namespaces[req.Name]; exists {
		http.Error(w, "命��空间已存在", http.StatusBadRequest)
		return
	}

	// 检查保留命名空间
	isReserved := false
	for _, reserved := range s.config.Namespace.Reserved {
		if strings.EqualFold(req.Name, reserved) {
			isReserved = true
			break
		}
	}
	if isReserved {
		http.Error(w, "命名空间被保留", http.StatusBadRequest)
		return
	}

	// 创建命名空间
	status := "active"
	if !s.config.Namespace.AutoApprove {
		status = "pending"
	}

	s.config.Namespaces[req.Name] = NS{
		Owners:      []string{username},
		Maintainers: []string{},
		Visibility:  "public",
		Status:      status,
	}

	// 保存配置（在锁内）
	configData := struct {
		Server     ServerConfig     `yaml:"server"`
		Namespace  NamespaceConfig  `yaml:"namespace"`
		Storage    StorageConfig    `yaml:"storage"`
		Users      map[string]User  `yaml:"users"`
		Namespaces map[string]NS    `yaml:"namespaces"`
		Upstreams  []UpstreamConfig `yaml:"upstreams"`
	}{
		Server:     s.config.Server,
		Namespace:  s.config.Namespace,
		Storage:    s.config.Storage,
		Users:      s.config.Users,
		Namespaces: s.config.Namespaces,
		Upstreams:  s.config.Upstreams,
	}

	data, err := yaml.Marshal(configData)
	if err != nil {
		http.Error(w, "保存配置失败", http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile("permissions.yaml", data, 0644); err != nil {
		http.Error(w, "保存配置失败", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":   req.Name,
		"status": status,
	})
}

// handlePackages 包管理
func (s *Server) handlePackages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.listPackages(w, r)
	case "POST":
		s.publishPackage(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listPackages 列出包
func (s *Server) listPackages(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")

	// 从本地存储读取 index.json
	index, err := s.loadLocalIndex()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"packages": []string{},
		})
		return
	}

	packages := []string{}
	for _, pkg := range index {
		if namespace != "" && !strings.HasPrefix(pkg, namespace+"/") {
			continue
		}
		packages = append(packages, pkg)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"packages": packages,
	})
}

// publishPackage 发布包
func (s *Server) publishPackage(w http.ResponseWriter, r *http.Request) {
	username, ok := s.auth.Authenticate(r)
	if !ok {
		http.Error(w, "未授权", http.StatusUnauthorized)
		return
	}

	// 解析 multipart form
	if err := r.ParseMultipartForm(s.config.Server.MaxUpload); err != nil {
		http.Error(w, "解析表单失败", http.StatusBadRequest)
		return
	}

	namespace := r.FormValue("namespace")
	name := r.FormValue("name")
	version := r.FormValue("version")

	if namespace == "" || name == "" || version == "" {
		http.Error(w, "缺少必要参数", http.StatusBadRequest)
		return
	}

	// 检查发布权限
	if !s.permission.CanPublish(username, namespace) {
		http.Error(w, "无权限发布到此命名空间", http.StatusForbidden)
		return
	}

	// 保存文件
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "读取文件失败", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "读取文件失败", http.StatusInternalServerError)
		return
	}

	pkgPath := fmt.Sprintf("packages/%s/%s/%s/%s", namespace, name, version, header.Filename)
	if err := s.storage.Save(pkgPath, data); err != nil {
		http.Error(w, "保存文件失败", http.StatusInternalServerError)
		return
	}

	// 更新索引
	pkgFullName := fmt.Sprintf("%s/%s", namespace, name)
	if err := s.updateIndex(pkgFullName); err != nil {
		log.Printf("更新索引失败: %v", err)
		// 不阻断发布流程，只记录日志
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("%s/%s@%s 发布成功", namespace, name, version),
	})
}

// handlePackageDownload 下载包
func (s *Server) handlePackageDownload(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path // /packages/namespace/name/version/file.tar.gz

	// 提取命名空间
	parts := strings.Split(strings.TrimPrefix(path, "/packages/"), "/")
	if len(parts) < 2 {
		http.Error(w, "无效路径", http.StatusBadRequest)
		return
	}
	namespace := parts[0]

	// 检查读取权限
	username, _ := s.auth.Authenticate(r)
	if !s.permission.CanRead(username, namespace) {
		http.Error(w, "无权限访问", http.StatusForbidden)
		return
	}

	// 存储路径
	storagePath := strings.TrimPrefix(path, "/")

	// 尝试本地存储
	if s.storage.Exists(storagePath) {
		data, err := s.storage.Load(storagePath)
		if err == nil {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(data)
			return
		}
	}

	// 尝试上游
	for _, upstream := range s.config.Upstreams {
		upstreamURL := strings.TrimSuffix(upstream.URL, "/") + path
		resp, err := s.client.Get(upstreamURL)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				continue
			}

			// 缓存到本地
			if upstream.Cache {
				s.storage.Save(storagePath, data)
			}

			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(data)
			return
		}
	}

	http.NotFound(w, r)
}

// handleProxy 代理其他请求
func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	// 尝试本地存储
	if s.storage.Exists(path) {
		data, err := s.storage.Load(path)
		if err == nil {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(data)
			return
		}
	}

	// 尝试上游
	for _, upstream := range s.config.Upstreams {
		upstreamURL := strings.TrimSuffix(upstream.URL, "/") + "/" + path
		resp, err := s.client.Get(upstreamURL)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				continue
			}

			// 缓存到本地
			if upstream.Cache {
				s.storage.Save(path, data)
			}

			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(data)
			return
		}
	}

	http.NotFound(w, r)
}

// loadLocalIndex 加载本地索引
func (s *Server) loadLocalIndex() ([]string, error) {
	data, err := s.storage.Load("index.json")
	if err != nil {
		return nil, err
	}

	var index struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}

	return index.Packages, nil
}

// saveConfig 保存配置
func (s *Server) saveConfig() error {
	s.config.mu.Lock()
	defer s.config.mu.Unlock()

	// 只序列化需要的字段（排除 sync.RWMutex）
	configData := struct {
		Server     ServerConfig     `yaml:"server"`
		Namespace  NamespaceConfig  `yaml:"namespace"`
		Storage    StorageConfig    `yaml:"storage"`
		Users      map[string]User  `yaml:"users"`
		Namespaces map[string]NS    `yaml:"namespaces"`
		Upstreams  []UpstreamConfig `yaml:"upstreams"`
	}{
		Server:     s.config.Server,
		Namespace:  s.config.Namespace,
		Storage:    s.config.Storage,
		Users:      s.config.Users,
		Namespaces: s.config.Namespaces,
		Upstreams:  s.config.Upstreams,
	}

	data, err := yaml.Marshal(configData)
	if err != nil {
		return err
	}

	return os.WriteFile("permissions.yaml", data, 0644)
}

// updateIndex 更新包索引
func (s *Server) updateIndex(pkgName string) error {
	// 加载现有索引
	packages, err := s.loadLocalIndex()
	if err != nil {
		packages = []string{}
	}

	// 检查是否已存在
	for _, p := range packages {
		if p == pkgName {
			return nil // 已存在，无需更新
		}
	}

	// 添加新包
	packages = append(packages, pkgName)

	// 保存索引
	indexData := struct {
		Packages []string `json:"packages"`
	}{
		Packages: packages,
	}

	data, err := json.Marshal(indexData)
	if err != nil {
		return err
	}

	return s.storage.Save("index.json", data)
}