package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gpu-pool/server/internal/node"
)

// 创建测试用的 HTTP Handler。
func newTestHandler() *Handler {
	return NewHandler(node.NewRegistry())
}

// 构造一份合法的节点心跳请求。
func validHeartbeatRequest() []byte {
	data := map[string]any{
		"node_id":      "node-001",
		"hostname":     "worker-01",
		"collected_at": time.Now().UTC(),
		"system":       map[string]any{},
		"gpus":         []any{},
	}

	body, _ := json.Marshal(data)
	return body
}

// 测试节点首次上报心跳。
func TestHeartbeatSuccess(t *testing.T) {
	handler := newTestHandler()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/heartbeat",
		bytes.NewReader(validHeartbeatRequest()),
	)

	rec := httptest.NewRecorder()

	handler.heartbeat(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s",
			rec.Code, rec.Body.String())
	}
}

// 测试非法 JSON 请求。
func TestHeartbeatInvalidJSON(t *testing.T) {
	handler := newTestHandler()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/heartbeat",
		bytes.NewBufferString("{invalid}"),
	)

	rec := httptest.NewRecorder()

	handler.heartbeat(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

// 测试缺少节点 ID 的请求。
func TestHeartbeatMissingNodeID(t *testing.T) {
	handler := newTestHandler()

	body := []byte(`{
		"hostname": "worker-01",
		"collected_at": "2026-11-02T00:00:00Z"
	}`)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/heartbeat",
		bytes.NewReader(body),
	)

	rec := httptest.NewRecorder()

	handler.heartbeat(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

// 测试节点列表查询。
func TestListNodes(t *testing.T) {
	handler := newTestHandler()

	heartbeatReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/heartbeat",
		bytes.NewReader(validHeartbeatRequest()),
	)

	heartbeatRec := httptest.NewRecorder()
	handler.heartbeat(heartbeatRec, heartbeatReq)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes",
		nil,
	)

	rec := httptest.NewRecorder()

	handler.listNodes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var result struct {
		Total int         `json:"total"`
		Nodes []node.Node `json:"nodes"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	if result.Total != 1 {
		t.Fatalf("expected 1 node, got %d", result.Total)
	}

	if result.Nodes[0].NodeID != "node-001" {
		t.Fatalf("unexpected node ID: %s", result.Nodes[0].NodeID)
	}
}

// 测试空节点列表。
func TestListNodesEmpty(t *testing.T) {
	handler := newTestHandler()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes",
		nil,
	)

	rec := httptest.NewRecorder()

	handler.listNodes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

// 测试查询已注册节点的详情。
func TestGetNodeSuccess(t *testing.T) {
	handler := newTestHandler()

	// 先注册节点。
	heartbeatReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/heartbeat",
		bytes.NewReader(validHeartbeatRequest()),
	)
	heartbeatRec := httptest.NewRecorder()
	handler.heartbeat(heartbeatRec, heartbeatReq)

	// 查询节点详情。
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes/node-001",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s",
			rec.Code, rec.Body.String())
	}

	var result struct {
		Node node.Node `json:"node"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	if result.Node.NodeID != "node-001" {
		t.Fatalf("unexpected node ID: %s", result.Node.NodeID)
	}

	if result.Node.Status != node.StatusOnline {
		t.Fatalf("expected online, got %s", result.Node.Status)
	}
}

// 测试查询不存在的节点返回 404。
func TestGetNodeNotFound(t *testing.T) {
	handler := newTestHandler()

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes/unknown-node",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

// 测试节点状态统计。
func TestNodeStats(t *testing.T) {
	handler := newTestHandler()

	// 注册一个节点。
	heartbeatReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/heartbeat",
		bytes.NewReader(validHeartbeatRequest()),
	)

	heartbeatRec := httptest.NewRecorder()
	handler.heartbeat(heartbeatRec, heartbeatReq)

	// 查询节点统计。
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/nodes/stats",
		nil,
	)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s",
			rec.Code, rec.Body.String())
	}

	var result node.NodeStats

	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	if result.Total != 1 {
		t.Fatalf("expected total 1, got %d", result.Total)
	}

	if result.Online != 1 {
		t.Fatalf("expected online 1, got %d", result.Online)
	}

	if result.Offline != 0 {
		t.Fatalf("expected offline 0, got %d", result.Offline)
	}
}

// 测试空节点状态统计。
func TestNodeStatsEmpty(t *testing.T) {
	handler := newTestHandler()

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

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

	var result node.NodeStats

	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	if result.Total != 0 || result.Online != 0 || result.Offline != 0 {
		t.Fatalf("expected all stats to be zero, got %+v", result)
	}
}
