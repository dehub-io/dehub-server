package main

import (
	"flag"
	"log"
)

func main() {
	configPath := flag.String("config", "permissions.yaml", "配置文件路径")
	flag.Parse()

	config, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	server := NewServer(config)
	if err := server.Start(); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}
}