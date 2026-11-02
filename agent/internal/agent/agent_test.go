package agent

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"gpu-pool/agent/internal/config"
	"gpu-pool/agent/internal/gpu"
)

// heartbeatTestGPUCollector is a mock GPU collector.
type heartbeatTestGPUCollector struct{}

func (c *heartbeatTestGPUCollector) Collect() ([]gpu.GPUInfo, error) {
	return nil, nil
}

// recordingHeartbeatClient records heartbeat requests.
type recordingHeartbeatClient struct {
	calls     atomic.Int32
	snapshots chan *Snapshot
	failFirst bool
}

func (c *recordingHeartbeatClient) Send(
	ctx context.Context,
	payload any,
) error {
	call := c.calls.Add(1)

	if c.failFirst && call == 1 {
		return errors.New("simulated heartbeat failure")
	}

	snapshot, ok := payload.(*Snapshot)
	if !ok {
		return fmt.Errorf("unexpected payload type: %T", payload)
	}

	select {
	case c.snapshots <- snapshot:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestNewAgent(t *testing.T) {
	cfg := config.DefaultConfig()

	agent, err := New(&cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if agent == nil {
		t.Fatal("agent should not be nil")
	}

	if agent.identity == nil {
		t.Fatal("identity should not be nil")
	}

	if agent.collector == nil {
		t.Fatal("collector should not be nil")
	}

	if agent.heartbeatClient == nil {
		t.Fatal("heartbeat client should not be nil")
	}
}

func TestNewAgentNilConfig(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Fatal("expected nil config error")
	}
}

func TestAgentRunCanceledContext(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Monitor.CollectionInterval = time.Second

	agent, err := New(&cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)

	go func() {
		done <- agent.Run(ctx)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run() did not stop after context cancellation")
	}
}

func TestAgentRunHeartbeatAndRecovery(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Monitor.CollectionInterval = time.Hour
	cfg.Heartbeat.Interval = 200 * time.Millisecond

	client := &recordingHeartbeatClient{
		snapshots: make(chan *Snapshot, 1),
		failFirst: true,
	}

	agent := &Agent{
		config: &cfg,
		collector: NewCollector(
			"test-node-uuid",
			"test-host",
			&heartbeatTestGPUCollector{},
		),
		heartbeatClient: client,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- agent.Run(ctx)
	}()

	// Wait for a successful heartbeat.
	var snapshot *Snapshot

	select {
	case snapshot = <-client.snapshots:
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("heartbeat was not sent successfully")
	}

	// Verify snapshot contents.
	if snapshot.NodeID != "test-node-uuid" {
		t.Errorf("unexpected node ID: %s", snapshot.NodeID)
	}

	if snapshot.Hostname != "test-host" {
		t.Errorf("unexpected hostname: %s", snapshot.Hostname)
	}

	if snapshot.CollectedAt.IsZero() {
		t.Error("snapshot collection time should not be zero")
	}

	// First heartbeat failed, second heartbeat succeeded.
	if client.calls.Load() < 2 {
		t.Errorf(
			"expected at least 2 heartbeat attempts, got %d",
			client.calls.Load(),
		)
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Agent did not stop after context cancellation")
	}
}
