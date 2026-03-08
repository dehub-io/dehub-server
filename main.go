package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	configPath := flag.String("config", "repository.yaml", "path to config file")
	flag.Parse()

	config, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	server := NewServer(config)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		server.Stop()
		os.Exit(0)
	}()

	fmt.Printf("Starting dehub-server on %s\n", config.Server.Listen)
	fmt.Printf("Repository root: %s\n", config.Server.Storage)
	if config.Upstream.URL != "" {
		fmt.Printf("Upstream: %s\n", config.Upstream.URL)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
