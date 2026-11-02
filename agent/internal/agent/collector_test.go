package agent

import (
	"errors"
	"testing"

	"gpu-pool/agent/internal/gpu"
)

type mockGPUCollector struct {
	gpus []gpu.GPUInfo
	err  error
}

func (m *mockGPUCollector) Collect() ([]gpu.GPUInfo, error) {
	return m.gpus, m.err
}

func TestCollectorCollect(t *testing.T) {
	mock := &mockGPUCollector{
		gpus: []gpu.GPUInfo{},
	}

	collector := NewCollector(
		"test-node-id",
		"test-host",
		mock,
	)

	snapshot, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if snapshot.NodeID != "test-node-id" {
		t.Errorf("unexpected node ID: %s", snapshot.NodeID)
	}

	if snapshot.Hostname != "test-host" {
		t.Errorf("unexpected hostname: %s", snapshot.Hostname)
	}

	if snapshot.CollectedAt.IsZero() {
		t.Error("collection timestamp should not be zero")
	}

	if snapshot.System.CPU.LogicalCores <= 0 {
		t.Error("CPU logical cores should be greater than zero")
	}
}

func TestCollectorGPUFailure(t *testing.T) {
	mock := &mockGPUCollector{
		err: errors.New("simulated GPU failure"),
	}

	collector := NewCollector(
		"test-node-id",
		"test-host",
		mock,
	)

	_, err := collector.Collect()
	if err == nil {
		t.Fatal("expected GPU collection error")
	}
}

func TestCollectorNilGPUCollector(t *testing.T) {
	collector := NewCollector(
		"test-node-id",
		"test-host",
		nil,
	)

	_, err := collector.Collect()
	if err == nil {
		t.Fatal("expected nil GPU collector error")
	}
}
