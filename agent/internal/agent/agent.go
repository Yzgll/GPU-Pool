package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gpu-pool/agent/internal/config"
	"gpu-pool/agent/internal/executor"
	"gpu-pool/agent/internal/gpu"
	"gpu-pool/agent/internal/heartbeat"
	"gpu-pool/agent/internal/identity"
)

// heartbeatSender defines the heartbeat sending interface.
type heartbeatSender interface {
	Send(ctx context.Context, snapshot any) error
}

// Agent manages the worker node lifecycle.
type Agent struct {
	config          *config.Config
	identity        *identity.Identity
	collector       *Collector
	heartbeatClient heartbeatSender

	snapshotMu     sync.RWMutex
	latestSnapshot *Snapshot
}

// New creates and initializes an Agent.
func New(cfg *config.Config) (*Agent, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	identityPath := filepath.Join(cfg.Agent.DataDir, "node.json")

	nodeIdentity, err := identity.LoadOrCreate(identityPath)
	if err != nil {
		return nil, fmt.Errorf("initialize identity: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("get hostname: %w", err)
	}

	if cfg.Agent.NodeName != "" {
		hostname = cfg.Agent.NodeName
	}

	gpuCollector, err := gpu.NewCollector()
	if err != nil {
		return nil, fmt.Errorf("initialize GPU collector: %w", err)
	}

	collector := NewCollector(
		nodeIdentity.NodeID,
		hostname,
		gpuCollector,
	)

	heartbeatClient, err := heartbeat.NewClient(
		cfg.Coordinator.Endpoint,
		5*time.Second,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize heartbeat client: %w", err)
	}

	return &Agent{
		config:          cfg,
		identity:        nodeIdentity,
		collector:       collector,
		heartbeatClient: heartbeatClient,
	}, nil
}

// Run starts resource collection and heartbeat reporting.
func (a *Agent) Run(ctx context.Context) error {
	if a == nil {
		return fmt.Errorf("agent cannot be nil")
	}

	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}

	if err := ctx.Err(); err != nil {
		return nil
	}

	if a.config == nil {
		return fmt.Errorf("agent config cannot be nil")
	}

	if a.collector == nil {
		return fmt.Errorf("agent collector cannot be nil")
	}

	if a.heartbeatClient == nil {
		return fmt.Errorf("heartbeat client cannot be nil")
	}

	collectionInterval := a.config.Monitor.CollectionInterval
	heartbeatInterval := a.config.Heartbeat.Interval

	if collectionInterval <= 0 {
		return fmt.Errorf("collection interval must be greater than zero")
	}

	if heartbeatInterval <= 0 {
		return fmt.Errorf("heartbeat interval must be greater than zero")
	}

	var wg sync.WaitGroup

	// Resource collection task.
	wg.Add(1)
	go func() {
		defer wg.Done()

		err := executor.Run(ctx, collectionInterval, func() error {
			snapshot, err := a.collector.Collect()
			if err != nil {
				return err
			}

			a.snapshotMu.Lock()
			a.latestSnapshot = snapshot
			a.snapshotMu.Unlock()

			return nil
		})

		if err != nil {
			log.Printf("Resource collection task stopped: %v", err)
		}
	}()

	// Heartbeat reporting task.
	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				a.snapshotMu.RLock()
				snapshot := a.latestSnapshot
				a.snapshotMu.RUnlock()

				if snapshot == nil {
					log.Printf("Heartbeat skipped: no resource snapshot available")
					continue
				}

				if err := a.heartbeatClient.Send(ctx, snapshot); err != nil {
					log.Printf("Heartbeat failed: %v", err)
					continue
				}

				log.Printf(
					"Heartbeat sent successfully: node_id=%s",
					snapshot.NodeID,
				)
			}
		}
	}()

	<-ctx.Done()
	wg.Wait()

	return nil
}
