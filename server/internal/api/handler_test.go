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
	nodes      []node.Node
	history    *node.ResourceHistory
	latest     *node.ResourceSnapshot
	historyErr error
	latestErr  error

	gpus          []node.GPUDevice
	gpusErr       error
	gpuHistory    *node.GPUHistoryResult
	gpuHistoryErr error
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

// GetResourceHistory 模拟资源历史查询。
func (m *mockNodeRepository) GetResourceHistory(
	ctx context.Context,
	nodeID string,
	from *time.Time,
	to *time.Time,
	limit int,
	offset int,
) (*node.ResourceHistory, error) {
	if m.historyErr != nil {
		return nil, m.historyErr
	}

	return m.history, nil
}

// GetLatestResourceSnapshot 模拟最新快照查询。
func (m *mockNodeRepository) GetLatestResourceSnapshot(
	ctx context.Context,
	nodeID string,
) (*node.ResourceSnapshot, error) {
	if m.latestErr != nil {
		return nil, m.latestErr
	}

	return m.latest, nil
}

func (m *mockNodeRepository) GetGPUHistory(
	ctx context.Context,
	nodeID string,
	gpuIndex *int,
	from *time.Time,
	to *time.Time,
	limit int,
	offset int,
) (*node.GPUHistoryResult, error) {
	if m.gpuHistoryErr != nil {
		return nil, m.gpuHistoryErr
	}

	return m.gpuHistory, nil
}

// ListGPUs 模拟查询所有 GPU。
func (m *mockNodeRepository) ListGPUs(
	ctx context.Context,
) ([]node.GPUDevice, error) {
	if m.gpusErr != nil {
		return nil, m.gpusErr
	}

	return m.gpus, nil
}

// GetGPUsByNode 模拟查询指定节点的 GPU。
func (m *mockNodeRepository) GetGPUsByNode(
	ctx context.Context,
	nodeID string,
) ([]node.GPUDevice, error) {
	if m.gpusErr != nil {
		return nil, m.gpusErr
	}

	result := make([]node.GPUDevice, 0)

	for _, gpu := range m.gpus {
		if gpu.NodeID == nodeID {
			result = append(result, gpu)
		}
	}

	return result, nil
}

// newTestServer 创建使用 Mock Repository 的测试服务。
func newTestServer() *http.ServeMux {
	return newTestServerWithRepository(&mockNodeRepository{})
}

// newTestServerWithRepository 支持注入不同的 Mock 数据。
func newTestServerWithRepository(repository NodeRepository) *http.ServeMux {
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
func TestGetResourceHistory(t *testing.T) {
	collectedAt := time.Date(
		2026, 11, 4, 14, 0, 0, 0, time.UTC,
	)

	repository := &mockNodeRepository{
		nodes: []node.Node{
			{
				NodeID:   "node-001",
				Hostname: "worker-001",
			},
		},
		history: &node.ResourceHistory{
			Total:  1,
			Limit:  20,
			Offset: 0,
			Records: []node.ResourceSnapshot{
				{
					ID:          1,
					NodeID:      "node-001",
					CollectedAt: collectedAt,
					SystemInfo:  json.RawMessage(`{"cpu":{}}`),
					GPUInfo:     json.RawMessage(`[]`),
				},
			},
		},
	}

	mux := newTestServerWithRepository(repository)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes/node-001/history?limit=20&offset=0",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s",
			rec.Code, rec.Body.String())
	}

	var result node.ResourceHistory
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result.Total != 1 || len(result.Records) != 1 {
		t.Fatalf("unexpected history result: %+v", result)
	}
}
func TestGetLatestResourceSnapshot(t *testing.T) {
	repository := &mockNodeRepository{
		latest: &node.ResourceSnapshot{
			ID:          1,
			NodeID:      "node-001",
			CollectedAt: time.Now().UTC(),
			SystemInfo:  json.RawMessage(`{"cpu":{}}`),
			GPUInfo:     json.RawMessage(`[]`),
		},
	}

	mux := newTestServerWithRepository(repository)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes/node-001/latest",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s",
			rec.Code, rec.Body.String())
	}

	var result struct {
		Snapshot node.ResourceSnapshot `json:"snapshot"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result.Snapshot.NodeID != "node-001" {
		t.Fatalf("unexpected node ID: %s", result.Snapshot.NodeID)
	}
}
func TestGetResourceHistoryInvalidLimit(t *testing.T) {
	repository := &mockNodeRepository{
		nodes: []node.Node{
			{NodeID: "node-001"},
		},
	}

	mux := newTestServerWithRepository(repository)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes/node-001/history?limit=500",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
func TestGetResourceHistoryNodeNotFound(t *testing.T) {
	mux := newTestServer()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes/not-exist/history",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}
func TestListGPUs(t *testing.T) {
	repository := &mockNodeRepository{
		gpus: []node.GPUDevice{
			{
				NodeID:        "node-001",
				Hostname:      "worker-001",
				GPUUUID:       "GPU-test-001",
				GPUIndex:      0,
				Name:          "NVIDIA GeForce RTX 3050",
				MemoryTotalMB: 4096,
				MemoryUsedMB:  1472,
				MemoryFreeMB:  2493,
				Utilization:   23,
				Temperature:   48,
				PowerW:        12.25,
				DriverVersion: "572.83",
			},
		},
	}

	handler := NewHandler(repository)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/gpus",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var result struct {
		Total int              `json:"total"`
		GPUs  []node.GPUDevice `json:"gpus"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result.Total != 1 {
		t.Fatalf("expected 1 GPU, got %d", result.Total)
	}

	if result.GPUs[0].Utilization != 23 {
		t.Fatalf("unexpected GPU utilization: %d", result.GPUs[0].Utilization)
	}
}

func TestGetNodeGPUs(t *testing.T) {
	repository := &mockNodeRepository{
		gpus: []node.GPUDevice{
			{
				NodeID:  "node-001",
				GPUUUID: "GPU-test-001",
				Name:    "RTX 3050",
			},
			{
				NodeID:  "node-002",
				GPUUUID: "GPU-test-002",
				Name:    "RTX 4090",
			},
		},
	}

	handler := NewHandler(repository)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes/node-001/gpus",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var result struct {
		NodeID string           `json:"node_id"`
		Total  int              `json:"total"`
		GPUs   []node.GPUDevice `json:"gpus"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result.NodeID != "node-001" {
		t.Fatalf("unexpected node ID: %s", result.NodeID)
	}

	if result.Total != 1 {
		t.Fatalf("expected 1 GPU, got %d", result.Total)
	}

	if result.GPUs[0].GPUUUID != "GPU-test-001" {
		t.Fatalf("unexpected GPU UUID: %s", result.GPUs[0].GPUUUID)
	}
}

func TestGetGPUHistory(t *testing.T) {
	repository := &mockNodeRepository{
		nodes: []node.Node{
			{
				NodeID:   "node-001",
				Hostname: "worker-001",
			},
		},
		gpuHistory: &node.GPUHistoryResult{
			Total:  1,
			Limit:  20,
			Offset: 0,
			Records: []node.GPUHistoryRecord{
				{
					ID:            1,
					NodeID:        "node-001",
					Hostname:      "worker-001",
					GPUUUID:       "GPU-test-001",
					GPUIndex:      0,
					Name:          "NVIDIA Test GPU",
					MemoryTotalMB: 8192,
					MemoryUsedMB:  4096,
					MemoryFreeMB:  4096,
					Utilization:   50,
					Temperature:   55,
					PowerW:        120.5,
					DriverVersion: "572.83",
					CollectedAt:   time.Now().UTC(),
				},
			},
		},
	}

	mux := http.NewServeMux()
	handler := NewHandler(repository)
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes/node-001/gpus/history?gpu_index=0&limit=20&offset=0",
		nil,
	)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s",
			rec.Code, rec.Body.String())
	}

	var result node.GPUHistoryResult

	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result.Total != 1 {
		t.Fatalf("expected 1 record, got %d", result.Total)
	}

	if len(result.Records) != 1 {
		t.Fatalf("expected 1 GPU history record, got %d",
			len(result.Records))
	}

	if result.Records[0].Utilization != 50 {
		t.Fatalf("expected utilization 50, got %d",
			result.Records[0].Utilization)
	}
}
