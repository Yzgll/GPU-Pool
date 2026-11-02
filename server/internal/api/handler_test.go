package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gpu-pool/server/internal/node"
)

// mockNodeRepository 模拟数据库 Repository。
type mockNodeRepository struct {
	nodes []node.Node
}

func (m *mockNodeRepository) UpsertNodeWithSnapshot(
	ctx context.Context,
	snapshot []byte,
) (*node.Node, error) {
	var metadata struct {
		NodeID      string    `json:"node_id"`
		Hostname    string    `json:"hostname"`
		CollectedAt time.Time `json:"collected_at"`
	}

	if err := json.Unmarshal(snapshot, &metadata); err != nil {
		return nil, err
	}

	if metadata.NodeID == "" || metadata.Hostname == "" ||
		metadata.CollectedAt.IsZero() {
		return nil, errors.New("invalid heartbeat metadata")
	}

	result := &node.Node{
		NodeID:        metadata.NodeID,
		Hostname:      metadata.Hostname,
		CollectedAt:   metadata.CollectedAt,
		LastHeartbeat: time.Now().UTC(),
		Status:        node.StatusOnline,
		Snapshot:      append(json.RawMessage(nil), snapshot...),
	}

	m.nodes = append(m.nodes, *result)

	return result, nil
}

func (m *mockNodeRepository) GetNode(
	ctx context.Context,
	nodeID string,
) (*node.Node, error) {
	for _, n := range m.nodes {
		if n.NodeID == nodeID {
			return &n, nil
		}
	}

	return nil, nil
}

func (m *mockNodeRepository) ListNodes(
	ctx context.Context,
) ([]node.Node, error) {
	return m.nodes, nil
}

func (m *mockNodeRepository) GetNodeStats(
	ctx context.Context,
) (node.NodeStats, error) {
	stats := node.NodeStats{
		Total: len(m.nodes),
	}

	for _, n := range m.nodes {
		if n.Status == node.StatusOnline {
			stats.Online++
		} else {
			stats.Offline++
		}
	}

	return stats, nil
}

// newTestServer 创建使用 Mock Repository 的测试服务。
func newTestServer() *http.ServeMux {
	repository := &mockNodeRepository{}
	handler := NewHandler(repository)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return mux
}

func TestHeartbeatSuccess(t *testing.T) {
	mux := newTestServer()

	body := `{
		"node_id": "node-001",
		"hostname": "worker-001",
		"collected_at": "2026-11-02T14:00:00Z",
		"system": {},
		"gpus": []
	}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/heartbeat",
		strings.NewReader(body),
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s",
			rec.Code, rec.Body.String())
	}

	var result map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result["status"] != "ok" {
		t.Fatalf("unexpected response: %v", result)
	}

	if result["node_id"] != "node-001" {
		t.Fatalf("unexpected node ID: %s", result["node_id"])
	}
}

func TestHeartbeatInvalidJSON(t *testing.T) {
	mux := newTestServer()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/heartbeat",
		strings.NewReader(`invalid json`),
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHeartbeatMissingNodeID(t *testing.T) {
	mux := newTestServer()

	body := `{
		"hostname": "worker-001",
		"collected_at": "2026-11-02T14:00:00Z",
		"system": {},
		"gpus": []
	}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/heartbeat",
		strings.NewReader(body),
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestListNodes(t *testing.T) {
	mux := newTestServer()

	body := `{
		"node_id": "node-001",
		"hostname": "worker-001",
		"collected_at": "2026-11-02T14:00:00Z",
		"system": {},
		"gpus": []
	}`

	heartbeat := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/heartbeat",
		strings.NewReader(body),
	)
	mux.ServeHTTP(httptest.NewRecorder(), heartbeat)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var result struct {
		Total int         `json:"total"`
		Nodes []node.Node `json:"nodes"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result.Total != 1 {
		t.Fatalf("expected 1 node, got %d", result.Total)
	}
}

func TestListNodesEmpty(t *testing.T) {
	mux := newTestServer()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var result struct {
		Total int `json:"total"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result.Total != 0 {
		t.Fatalf("expected 0 nodes, got %d", result.Total)
	}
}

func TestGetNodeSuccess(t *testing.T) {
	mux := newTestServer()

	body := `{
		"node_id": "node-001",
		"hostname": "worker-001",
		"collected_at": "2026-11-02T14:00:00Z",
		"system": {},
		"gpus": []
	}`

	heartbeat := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/heartbeat",
		strings.NewReader(body),
	)
	mux.ServeHTTP(httptest.NewRecorder(), heartbeat)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes/node-001",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var result struct {
		Node node.Node `json:"node"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result.Node.NodeID != "node-001" {
		t.Fatalf("unexpected node ID: %s", result.Node.NodeID)
	}
}

func TestGetNodeNotFound(t *testing.T) {
	mux := newTestServer()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes/not-exist",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestNodeStats(t *testing.T) {
	mux := newTestServer()

	body := `{
		"node_id": "node-001",
		"hostname": "worker-001",
		"collected_at": "2026-11-02T14:00:00Z",
		"system": {},
		"gpus": []
	}`

	heartbeat := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/heartbeat",
		strings.NewReader(body),
	)
	mux.ServeHTTP(httptest.NewRecorder(), heartbeat)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes/stats",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var stats node.NodeStats
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}

	if stats.Total != 1 || stats.Online != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestNodeStatsEmpty(t *testing.T) {
	mux := newTestServer()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes/stats",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var stats node.NodeStats
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}

	if stats.Total != 0 {
		t.Fatalf("expected 0 nodes, got %d", stats.Total)
	}
}
