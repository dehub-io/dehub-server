package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Server struct {
	config *Config
	http   *http.Server
	client *http.Client
}

func NewServer(config *Config) *Server {
	return &Server{
		config: config,
		http: &http.Server{
			Addr: config.Server.Listen,
		},
		client: &http.Client{},
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)
	s.http.Handler = mux

	if err := os.MkdirAll(s.config.Server.Storage, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	return s.http.ListenAndServe()
}

func (s *Server) Stop() {
	s.http.Close()
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	localPath := filepath.Join(s.config.Server.Storage, path)

	if _, err := os.Stat(localPath); err == nil {
		http.ServeFile(w, r, localPath)
		return
	}

	if s.config.Upstream.Enabled && s.config.Upstream.URL != "" {
		s.serveFromUpstream(w, r, path)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) serveFromUpstream(w http.ResponseWriter, r *http.Request, path string) {
	upstreamURL := strings.TrimSuffix(s.config.Upstream.URL, "/") + path

	resp, err := s.client.Get(upstreamURL)
	if err != nil {
		log.Printf("Upstream error: %v", err)
		http.Error(w, "Upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Not found", resp.StatusCode)
		return
	}

	if s.config.Cache.Enabled {
		localPath := filepath.Join(s.config.Server.Storage, path)
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err == nil {
			if data, err := io.ReadAll(resp.Body); err == nil {
				os.WriteFile(localPath, data, 0644)
				http.ServeFile(w, r, localPath)
				return
			}
		}
	}

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	io.Copy(w, resp.Body)
}
