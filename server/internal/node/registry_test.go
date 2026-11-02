package node

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

// 创建一份合法的节点心跳数据，供各测试复用。
func validSnapshot(nodeID string) []byte {
	data := map[string]any{
		"node_id":      nodeID,
		"hostname":     "worker-01",
		"collected_at": time.Now().UTC(),
		"system":       map[string]any{},
		"gpus":         []any{},
	}

	raw, _ := json.Marshal(data)
	return raw
}

// 测试首次心跳能够成功注册节点。
func TestRegistryUpsert(t *testing.T) {
	registry := NewRegistry()

	n, err := registry.Upsert(validSnapshot("node-001"))
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	if n.NodeID != "node-001" {
		t.Fatalf("unexpected node ID: %s", n.NodeID)
	}

	if n.Hostname != "worker-01" {
		t.Fatalf("unexpected hostname: %s", n.Hostname)
	}

	if n.LastHeartbeat.IsZero() {
		t.Fatal("last heartbeat should not be zero")
	}

	if len(registry.GetAll()) != 1 {
		t.Fatal("expected one registered node")
	}
}

// 测试相同节点重复上报时更新数据，而不是创建新节点。
func TestRegistryUpsertUpdate(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Upsert(validSnapshot("node-001"))
	if err != nil {
		t.Fatal(err)
	}

	data := []byte(fmt.Sprintf(
		`{"node_id":"node-001","hostname":"worker-02","collected_at":%q}`,
		time.Now().UTC().Format(time.RFC3339Nano),
	))

	_, err = registry.Upsert(data)
	if err != nil {
		t.Fatal(err)
	}

	nodes := registry.GetAll()

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	if nodes[0].Hostname != "worker-02" {
		t.Fatalf("hostname was not updated: %s", nodes[0].Hostname)
	}
}

// 测试非法心跳数据被拒绝。
func TestRegistryUpsertInvalid(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "invalid JSON",
			data: []byte(`{invalid}`),
		},
		{
			name: "empty node ID",
			data: []byte(`{"hostname":"worker-01","collected_at":"2026-11-02T00:00:00Z"}`),
		},
		{
			name: "empty hostname",
			data: []byte(`{"node_id":"node-001","collected_at":"2026-11-02T00:00:00Z"}`),
		},
		{
			name: "empty collected time",
			data: []byte(`{"node_id":"node-001","hostname":"worker-01"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := registry.Upsert(tt.data)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

// 测试多个 goroutine 并发更新节点时不会发生数据竞争。
func TestRegistryConcurrentUpsert(t *testing.T) {
	registry := NewRegistry()

	const count = 100

	var wg sync.WaitGroup

	for i := 0; i < count; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			data := validSnapshot(fmt.Sprintf("node-%03d", i))
			if _, err := registry.Upsert(data); err != nil {
				t.Errorf("Upsert failed: %v", err)
			}
		}(i)
	}

	wg.Wait()

	if got := len(registry.GetAll()); got != count {
		t.Fatalf("expected %d nodes, got %d", count, got)
	}
}

// 测试节点心跳后状态为 online。
func TestRegistryNodeOnline(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Upsert(validSnapshot("node-001"))
	if err != nil {
		t.Fatal(err)
	}

	n, exists := registry.GetByID("node-001")
	if !exists {
		t.Fatal("node not found")
	}

	if n.Status != StatusOnline {
		t.Fatalf("expected online, got %s", n.Status)
	}
}

// 测试超过 30 秒没有心跳后节点变为 offline。
func TestRegistryNodeOffline(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Upsert(validSnapshot("node-001"))
	if err != nil {
		t.Fatal(err)
	}

	registry.mu.Lock()
	n := registry.nodes["node-001"]
	n.LastHeartbeat = time.Now().Add(-OfflineAfter - time.Second)
	registry.nodes["node-001"] = n
	registry.mu.Unlock()

	got, exists := registry.GetByID("node-001")
	if !exists {
		t.Fatal("node not found")
	}

	if got.Status != StatusOffline {
		t.Fatalf("expected offline, got %s", got.Status)
	}
}

// 测试查询不存在的节点。
func TestRegistryGetByIDNotFound(t *testing.T) {
	registry := NewRegistry()

	_, exists := registry.GetByID("unknown-node")
	if exists {
		t.Fatal("expected node not found")
	}
}

// 测试节点离线后重新上报心跳能够恢复在线状态。
func TestRegistryNodeRecovery(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Upsert(validSnapshot("node-001"))
	if err != nil {
		t.Fatal(err)
	}

	registry.mu.Lock()
	n := registry.nodes["node-001"]
	n.LastHeartbeat = time.Now().Add(-OfflineAfter - time.Second)
	registry.nodes["node-001"] = n
	registry.mu.Unlock()

	_, err = registry.Upsert(validSnapshot("node-001"))
	if err != nil {
		t.Fatal(err)
	}

	got, exists := registry.GetByID("node-001")
	if !exists {
		t.Fatal("node not found")
	}

	if got.Status != StatusOnline {
		t.Fatalf("expected online after recovery, got %s", got.Status)
	}
}
