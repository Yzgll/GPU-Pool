package api

import (
	"encoding/json"
	"io"
	"net/http"

	"gpu-pool/server/internal/node"
)

type Handler struct {
	registry *node.Registry
}

func NewHandler(registry *node.Registry) *Handler {
	return &Handler{
		registry: registry,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/heartbeat", h.heartbeat)
	mux.HandleFunc("GET /api/v1/nodes", h.listNodes)
	mux.HandleFunc("GET /api/v1/nodes/{node_id}", h.getNode)
	mux.HandleFunc("GET /api/v1/nodes/stats", h.nodeStats)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *Handler) heartbeat(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.registry == nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "node registry unavailable"})
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

	n, err := h.registry.Upsert(body)
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

func (h *Handler) listNodes(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.registry == nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "node registry unavailable"})
		return
	}

	nodes := h.registry.GetAll()

	writeJSON(w, http.StatusOK, map[string]any{
		"total": len(nodes),
		"nodes": nodes,
	})
}

// nodeStats 返回节点总数、在线数和离线数。
func (h *Handler) nodeStats(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.registry == nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "node registry unavailable"})
		return
	}

	stats := h.registry.GetStats()

	writeJSON(w, http.StatusOK, stats)
}

// getNode 根据 URL 中的 node_id 返回节点详情。
func (h *Handler) getNode(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.registry == nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "node registry unavailable"})
		return
	}

	nodeID := r.PathValue("node_id")
	if nodeID == "" {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "node_id cannot be empty"})
		return
	}

	n, exists := h.registry.GetByID(nodeID)
	if !exists {
		writeJSON(w, http.StatusNotFound,
			map[string]string{"error": "node not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"node": n,
	})
}
