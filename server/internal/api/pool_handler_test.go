package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gpu-pool/server/internal/node"
)

func newPoolTestRepository() *mockNodeRepository {
	return &mockNodeRepository{
		nodes: []node.Node{
			{
				NodeID:   "node-001",
				Hostname: "worker-001",
				Status:   node.StatusOnline,
			},
			{
				NodeID:   "node-002",
				Hostname: "worker-002",
				Status:   node.StatusOffline,
			},
		},
		pools: []node.Pool{
			{
				PoolID: 1,
				Name:   "test-pool",
				Status: "active",
			},
		},
		nextPoolID: 1,
		gpus: []node.GPUDevice{
			{
				NodeID:        "node-001",
				MemoryTotalMB: 16384,
				MemoryUsedMB:  4096,
				MemoryFreeMB:  12288,
			},
		},
		poolStats: make(map[int64]node.PoolStats),
	}
}

func performPoolRequest(
	mux *http.ServeMux,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestCreatePoolHandler(t *testing.T) {
	repo := newPoolTestRepository()
	mux := newTestServerWithRepository(repo)

	rec := performPoolRequest(
		mux, http.MethodPost, "/api/v1/pools",
		`{"name":"new-pool","description":"test description"}`,
	)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	if len(repo.pools) != 2 {
		t.Fatalf("expected 2 pools, got %d", len(repo.pools))
	}

	tests := []struct {
		name string
		body string
	}{
		{"empty name", `{"name":""}`},
		{"invalid JSON", `{invalid}`},
		{"unknown field", `{"name":"pool","unknown":"value"}`},
		{"duplicate name", `{"name":"test-pool"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := performPoolRequest(
				mux, http.MethodPost, "/api/v1/pools", tt.body,
			)
			if rec.Code < 400 {
				t.Fatalf("expected error status, got %d", rec.Code)
			}
		})
	}
}

func TestListPoolsHandler(t *testing.T) {
	mux := newTestServerWithRepository(newPoolTestRepository())

	rec := performPoolRequest(
		mux, http.MethodGet, "/api/v1/pools", "",
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"total":1`) {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

func TestGetPoolHandler(t *testing.T) {
	mux := newTestServerWithRepository(newPoolTestRepository())

	rec := performPoolRequest(
		mux, http.MethodGet, "/api/v1/pools/1", "",
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"test-pool"`) {
		t.Fatalf("pool not found in response: %s", rec.Body.String())
	}

	rec = performPoolRequest(
		mux, http.MethodGet, "/api/v1/pools/invalid", "",
	)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdatePoolHandler(t *testing.T) {
	mux := newTestServerWithRepository(newPoolTestRepository())

	rec := performPoolRequest(
		mux, http.MethodPut, "/api/v1/pools/1",
		`{"name":"updated-pool","description":"updated"}`,
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = performPoolRequest(
		mux, http.MethodPut, "/api/v1/pools/1",
		`{"name":""}`,
	)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeletePoolHandler(t *testing.T) {
	repo := newPoolTestRepository()
	repo.poolNodes = []node.PoolNode{
		{PoolID: 1, NodeID: "node-001"},
	}

	mux := newTestServerWithRepository(repo)

	rec := performPoolRequest(
		mux, http.MethodDelete, "/api/v1/pools/1", "",
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if len(repo.pools) != 0 {
		t.Fatal("pool was not deleted")
	}

	if len(repo.poolNodes) != 0 {
		t.Fatal("pool-node associations were not removed")
	}

	if len(repo.nodes) != 2 {
		t.Fatal("deleting pool must not delete worker nodes")
	}
}

func TestSetPoolStatusHandler(t *testing.T) {
	repo := newPoolTestRepository()
	mux := newTestServerWithRepository(repo)

	rec := performPoolRequest(
		mux, http.MethodPatch, "/api/v1/pools/1/status",
		`{"status":"disabled"}`,
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if repo.pools[0].Status != "disabled" {
		t.Fatal("pool status was not updated")
	}

	rec = performPoolRequest(
		mux, http.MethodPatch, "/api/v1/pools/1/status",
		`{"status":"invalid"}`,
	)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAddNodeToPoolHandler(t *testing.T) {
	repo := newPoolTestRepository()
	mux := newTestServerWithRepository(repo)

	rec := performPoolRequest(
		mux, http.MethodPost, "/api/v1/pools/1/nodes",
		`{"node_id":"node-001"}`,
	)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	if len(repo.poolNodes) != 1 {
		t.Fatalf("expected 1 pool node, got %d", len(repo.poolNodes))
	}

	// 一个节点不能重复加入资源池。
	rec = performPoolRequest(
		mux, http.MethodPost, "/api/v1/pools/1/nodes",
		`{"node_id":"node-001"}`,
	)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}

	// 离线节点不能加入资源池。
	rec = performPoolRequest(
		mux, http.MethodPost, "/api/v1/pools/1/nodes",
		`{"node_id":"node-002"}`,
	)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}

	// 不存在的节点。
	rec = performPoolRequest(
		mux, http.MethodPost, "/api/v1/pools/1/nodes",
		`{"node_id":"node-999"}`,
	)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestAddNodeToDisabledPoolHandler(t *testing.T) {
	repo := newPoolTestRepository()
	repo.pools[0].Status = "disabled"

	mux := newTestServerWithRepository(repo)

	rec := performPoolRequest(
		mux, http.MethodPost, "/api/v1/pools/1/nodes",
		`{"node_id":"node-001"}`,
	)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestRemoveNodeFromPoolHandler(t *testing.T) {
	repo := newPoolTestRepository()
	repo.poolNodes = []node.PoolNode{
		{PoolID: 1, NodeID: "node-001"},
	}

	mux := newTestServerWithRepository(repo)

	rec := performPoolRequest(
		mux, http.MethodDelete,
		"/api/v1/pools/1/nodes/node-001", "",
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if len(repo.poolNodes) != 0 {
		t.Fatal("node was not removed from pool")
	}

	// 移除后节点仍然存在。
	if len(repo.nodes) != 2 {
		t.Fatal("removing pool association must not delete node")
	}

	// 再次移除不存在的关联。
	rec = performPoolRequest(
		mux, http.MethodDelete,
		"/api/v1/pools/1/nodes/node-001", "",
	)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestListPoolNodesHandler(t *testing.T) {
	repo := newPoolTestRepository()
	repo.poolNodes = []node.PoolNode{
		{
			PoolID:   1,
			NodeID:   "node-001",
			Hostname: "worker-001",
		},
	}

	mux := newTestServerWithRepository(repo)

	rec := performPoolRequest(
		mux, http.MethodGet, "/api/v1/pools/1/nodes", "",
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"worker-001"`) {
		t.Fatalf("node missing from response: %s", rec.Body.String())
	}
}

func TestGetPoolStatsHandler(t *testing.T) {
	repo := newPoolTestRepository()
	repo.poolNodes = []node.PoolNode{
		{PoolID: 1, NodeID: "node-001"},
	}

	mux := newTestServerWithRepository(repo)

	rec := performPoolRequest(
		mux, http.MethodGet, "/api/v1/pools/1/stats", "",
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()

	for _, expected := range []string{
		`"node_count":1`,
		`"gpu_count":1`,
		`"memory_total_mb":16384`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %s in response: %s", expected, body)
		}
	}
}

func TestPoolHandlerRepositoryError(t *testing.T) {
	repo := newPoolTestRepository()
	repo.poolErr = errors.New("database unavailable")

	mux := newTestServerWithRepository(repo)

	rec := performPoolRequest(
		mux, http.MethodGet, "/api/v1/pools", "",
	)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	if strings.Contains(rec.Body.String(), "database unavailable") {
		t.Fatal("internal error details should not be exposed")
	}
}
