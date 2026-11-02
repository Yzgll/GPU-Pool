package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"gpu-pool/agent/internal/agent"
	"gpu-pool/agent/internal/config"
)

const configPath = `E:\GPU-Pool-Data\config.yaml`

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// 1. 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Config loading failed: %v", err)
	}

	// 2. 创建模型缓存目录
	if err := os.MkdirAll(cfg.Storage.ModelCacheDir, 0755); err != nil {
		log.Fatalf("Create model cache directory failed: %v", err)
	}

	// 3. 初始化 Agent
	app, err := agent.New(cfg)
	if err != nil {
		log.Fatalf("Agent initialization failed: %v", err)
	}

	// 4. 监听退出信号，支持优雅退出
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	log.Println("GPU-Pool Worker Agent starting")
	log.Printf("Coordinator: %s", cfg.Coordinator.Endpoint)
	log.Printf("Collection interval: %s", cfg.Monitor.CollectionInterval)
	log.Printf("Heartbeat interval: %s", cfg.Heartbeat.Interval)
	log.Printf("Data directory: %s", cfg.Agent.DataDir)

	// 5. 启动资源采集与心跳任务
	if err := app.Run(ctx); err != nil {
		log.Fatalf("Agent execution failed: %v", err)
	}

	log.Println("GPU-Pool Worker Agent stopped gracefully")
}
