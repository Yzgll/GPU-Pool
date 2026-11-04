package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"gpu-pool/server/internal/node"
)

// NodeRepository 定义 API 所需的数据库操作。
// NodeRepository 定义 API 所需的数据库操作。
type NodeRepository interface {
	UpsertNodeWithSnapshot(ctx context.Context, snapshot []byte) (*node.Node, error)
	GetNode(ctx context.Context, nodeID string) (*node.Node, error)
	ListNodes(ctx context.Context) ([]node.Node, error)
	GetNodeStats(ctx context.Context) (node.NodeStats, error)
	// 资源历史查询。
	GetResourceHistory(ctx context.Context, nodeID string, from *time.Time, to *time.Time, limit int, offset int) (*node.ResourceHistory, error)
	GetLatestResourceSnapshot(ctx context.Context, nodeID string) (*node.ResourceSnapshot, error)
	ListGPUs(ctx context.Context) ([]node.GPUDevice, error)
	GetGPUsByNode(ctx context.Context, nodeID string) ([]node.GPUDevice, error)
	GetGPUHistory(
		ctx context.Context,
		nodeID string,
		gpuIndex *int,
		from *time.Time,
		to *time.Time,
		limit int,
		offset int,
	) (*node.GPUHistoryResult, error)
}

// Handler 负责处理 Coordinator HTTP 请求。
type Handler struct {
	repository NodeRepository
}

// NewHandler 创建 API Handler。
func NewHandler(repository NodeRepository) *Handler {
	return &Handler{repository: repository}
}

// RegisterRoutes 注册节点管理 API。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/heartbeat", h.heartbeat)
	mux.HandleFunc("GET /api/v1/nodes", h.listNodes)
	mux.HandleFunc("GET /api/v1/nodes/{node_id}", h.getNode)
	mux.HandleFunc("GET /api/v1/nodes/stats", h.nodeStats)
	mux.HandleFunc("GET /api/v1/nodes/{node_id}/history", h.getResourceHistory)
	mux.HandleFunc("GET /api/v1/nodes/{node_id}/latest", h.getLatestResourceSnapshot)
	mux.HandleFunc("GET /api/v1/gpus", h.listGPUs)
	mux.HandleFunc("GET /api/v1/nodes/{node_id}/gpus", h.getNodeGPUs)
	mux.HandleFunc("GET /api/v1/nodes/{node_id}/gpus/history", h.getGPUHistory)
}

// writeJSON 统一返回 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// heartbeat 接收 Agent 心跳并写入 PostgreSQL。
func (h *Handler) heartbeat(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repository == nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "node repository unavailable"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "failed to read request body"})
		return
	}

	n, err := h.repository.UpsertNodeWithSnapshot(r.Context(), body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"node_id": n.NodeID,
	})
}

// listNodes 从 PostgreSQL 查询全部节点。
func (h *Handler) listNodes(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repository == nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "node repository unavailable"})
		return
	}

	nodes, err := h.repository.ListNodes(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "failed to query nodes"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total": len(nodes),
		"nodes": nodes,
	})
}

// nodeStats 查询节点在线状态统计。
func (h *Handler) nodeStats(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repository == nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "node repository unavailable"})
		return
	}

	stats, err := h.repository.GetNodeStats(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "failed to query node stats"})
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// getNode 查询指定节点详情。
func (h *Handler) getNode(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repository == nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "node repository unavailable"})
		return
	}

	nodeID := r.PathValue("node_id")
	if nodeID == "" {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "node_id cannot be empty"})
		return
	}

	n, err := h.repository.GetNode(r.Context(), nodeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "failed to query node"})
		return
	}

	if n == nil {
		writeJSON(w, http.StatusNotFound,
			map[string]string{"error": "node not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"node": n,
	})
}

// getResourceHistory 分页查询指定节点的资源历史。
func (h *Handler) getResourceHistory(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repository == nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "node repository unavailable"})
		return
	}

	nodeID := r.PathValue("node_id")
	if nodeID == "" {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "node_id cannot be empty"})
		return
	}

	// 检查节点是否存在。
	n, err := h.repository.GetNode(r.Context(), nodeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "failed to query node"})
		return
	}

	if n == nil {
		writeJSON(w, http.StatusNotFound,
			map[string]string{"error": "node not found"})
		return
	}

	// 默认每页返回 20 条，最多 200 条。
	limit := 20
	offset := 0

	if value := r.URL.Query().Get("limit"); value != "" {
		limit, err = strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 200 {
			writeJSON(w, http.StatusBadRequest,
				map[string]string{"error": "limit must be between 1 and 200"})
			return
		}
	}

	if value := r.URL.Query().Get("offset"); value != "" {
		offset, err = strconv.Atoi(value)
		if err != nil || offset < 0 {
			writeJSON(w, http.StatusBadRequest,
				map[string]string{"error": "offset cannot be negative"})
			return
		}
	}

	// 时间参数使用 RFC3339 格式。
	var from *time.Time
	var to *time.Time

	if value := r.URL.Query().Get("from"); value != "" {
		parsed, parseErr := time.Parse(time.RFC3339, value)
		if parseErr != nil {
			writeJSON(w, http.StatusBadRequest,
				map[string]string{"error": "invalid from time"})
			return
		}
		from = &parsed
	}

	if value := r.URL.Query().Get("to"); value != "" {
		parsed, parseErr := time.Parse(time.RFC3339, value)
		if parseErr != nil {
			writeJSON(w, http.StatusBadRequest,
				map[string]string{"error": "invalid to time"})
			return
		}
		to = &parsed
	}

	if from != nil && to != nil && from.After(*to) {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "from cannot be after to"})
		return
	}

	history, err := h.repository.GetResourceHistory(
		r.Context(),
		nodeID,
		from,
		to,
		limit,
		offset,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "failed to query resource history"})
		return
	}

	writeJSON(w, http.StatusOK, history)
}

// getLatestResourceSnapshot 查询指定节点最新的资源快照。
func (h *Handler) getLatestResourceSnapshot(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repository == nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "node repository unavailable"})
		return
	}

	nodeID := r.PathValue("node_id")
	if nodeID == "" {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "node_id cannot be empty"})
		return
	}

	snapshot, err := h.repository.GetLatestResourceSnapshot(
		r.Context(),
		nodeID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "failed to query latest resource snapshot"})
		return
	}

	if snapshot == nil {
		writeJSON(w, http.StatusNotFound,
			map[string]string{"error": "resource snapshot not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"snapshot": snapshot,
	})
}

// listGPUs 查询所有节点的 GPU。
func (h *Handler) listGPUs(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repository == nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "node repository unavailable"})
		return
	}

	gpus, err := h.repository.ListGPUs(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "failed to query GPUs"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total": len(gpus),
		"gpus":  gpus,
	})
}

// getNodeGPUs 查询指定节点的 GPU。
func (h *Handler) getNodeGPUs(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repository == nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "node repository unavailable"})
		return
	}

	nodeID := r.PathValue("node_id")
	if nodeID == "" {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "node_id cannot be empty"})
		return
	}

	gpus, err := h.repository.GetGPUsByNode(r.Context(), nodeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "failed to query node GPUs"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"node_id": nodeID,
		"total":   len(gpus),
		"gpus":    gpus,
	})
}
