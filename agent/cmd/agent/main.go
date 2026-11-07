package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"gpu-pool/agent/internal/agent"
	"gpu-pool/agent/internal/config"
)

var (
	configPath = flag.String(
		"config",
		defaultConfigPath(),
		"Path to the Agent configuration file",
	)

	collectOnce = flag.Bool(
		"collect-once",
		false,
		"Collect resources once, print the snapshot as JSON, and exit",
	)
)

func defaultConfigPath() string {
	if runtime.GOOS == "windows" {
		return `E:\GPU-Pool-Data\config.yaml`
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}

	return filepath.Join(home, "GPU-Pool-Data", "config.yaml")
}

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// 1. 加载配置
	cfg, err := config.Load(*configPath)
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

	// 3.1 一次性采集模式：采集一次、打印 JSON、退出。
	if *collectOnce {
		snapshot, err := app.CollectOnce()
		if err != nil {
			log.Fatalf("Collect-once failed: %v", err)
		}

		data, err := json.MarshalIndent(snapshot, "", "  ")
		if err != nil {
			log.Fatalf("Marshal snapshot failed: %v", err)
		}

		os.Stdout.Write(data)
		os.Stdout.WriteString("\n")
		return
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
