package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"gpu-pool/agent/internal/config"
	"gpu-pool/agent/internal/executor"
	"gpu-pool/agent/internal/gpu"
	"gpu-pool/agent/internal/identity"
	"gpu-pool/agent/internal/monitor"
)

// AgentSnapshot represents the current worker node state.
type AgentSnapshot struct {
	NodeID      string             `json:"node_id"`
	Hostname    string             `json:"hostname"`
	CollectedAt time.Time          `json:"collected_at"`
	System      monitor.SystemInfo `json:"system"`
	GPUs        []gpu.GPUInfo      `json:"gpus"`
}

const configPath = `E:\GPU-Pool-Data\config.yaml`

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// 1. Load configuration.
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Config loading failed: %v", err)
	}

	// 2. Create model cache directory.
	if err := os.MkdirAll(cfg.Storage.ModelCacheDir, 0755); err != nil {
		log.Fatalf("Create model cache directory failed: %v", err)
	}

	// 3. Load or create persistent node identity.
	identityPath := filepath.Join(cfg.Agent.DataDir, "node.json")

	nodeIdentity, err := identity.LoadOrCreate(identityPath)
	if err != nil {
		log.Fatalf("Identity initialization failed: %v", err)
	}

	// 4. Get hostname.
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Get hostname failed: %v", err)
	}

	if cfg.Agent.NodeName != "" {
		hostname = cfg.Agent.NodeName
	}

	// 5. Initialize GPU collector.
	collector := gpu.NewNVIDIACollector()

	// 6. Prepare JSON encoder.
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")

	// 7. Define one resource collection operation.
	collect := func() error {
		// Collect GPU information.
		gpus, err := collector.Collect()
		if err != nil {
			return fmt.Errorf("GPU collection failed: %w", err)
		}

		// Collect system information.
		systemInfo, err := monitor.CollectSystemInfo()
		if err != nil {
			return fmt.Errorf("system collection failed: %w", err)
		}

		// Build resource snapshot.
		snapshot := AgentSnapshot{
			NodeID:      nodeIdentity.NodeID,
			Hostname:    hostname,
			CollectedAt: time.Now().UTC(),
			System:      *systemInfo,
			GPUs:        gpus,
		}

		// Output JSON snapshot.
		if err := encoder.Encode(snapshot); err != nil {
			return fmt.Errorf("JSON encoding failed: %w", err)
		}

		return nil
	}

	// 8. Handle Ctrl+C and termination signals.
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	log.Printf("GPU-Pool Worker Agent started")
	log.Printf("Node ID: %s", nodeIdentity.NodeID)
	log.Printf("Hostname: %s", hostname)
	log.Printf("Coordinator: %s", cfg.Coordinator.Endpoint)
	log.Printf("Collection interval: %s", cfg.Monitor.CollectionInterval)
	log.Printf("Data directory: %s", cfg.Agent.DataDir)

	// 9. Start periodic resource collection.
	if err := executor.Run(
		ctx,
		cfg.Monitor.CollectionInterval,
		collect,
	); err != nil {
		log.Printf("Agent execution failed: %v", err)
		os.Exit(1)
	}

	log.Printf("GPU-Pool Worker Agent stopped gracefully")
}
