package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"gpu-pool/server/internal/node"
)

// NodeRepository 定义 API 所需的数据库操作。
type NodeRepository interface {
	UpsertNodeWithSnapshot(ctx context.Context, snapshot []byte) (*node.Node, error)
	GetNode(ctx context.Context, nodeID string) (*node.Node, error)
	ListNodes(ctx context.Context) ([]node.Node, error)
	GetNodeStats(ctx context.Context) (node.NodeStats, error)
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
