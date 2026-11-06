package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"gpu-pool/server/internal/node"
)

type createPoolRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type updatePoolRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type updatePoolStatusRequest struct {
	Status string `json:"status"`
}

type addPoolNodeRequest struct {
	NodeID string `json:"node_id"`
}

// registerPoolRoutes 注册资源池 API。
func (h *Handler) registerPoolRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/pools", h.createPool)
	mux.HandleFunc("GET /api/v1/pools", h.listPools)
	mux.HandleFunc("GET /api/v1/pools/{pool_id}", h.getPool)
	mux.HandleFunc("PUT /api/v1/pools/{pool_id}", h.updatePool)
	mux.HandleFunc("DELETE /api/v1/pools/{pool_id}", h.deletePool)
	mux.HandleFunc("PATCH /api/v1/pools/{pool_id}/status", h.setPoolStatus)
	mux.HandleFunc("POST /api/v1/pools/{pool_id}/nodes", h.addNodeToPool)
	mux.HandleFunc("DELETE /api/v1/pools/{pool_id}/nodes/{node_id}", h.removeNodeFromPool)
	mux.HandleFunc("GET /api/v1/pools/{pool_id}/nodes", h.listPoolNodes)
	mux.HandleFunc("GET /api/v1/pools/{pool_id}/stats", h.getPoolStats)
}

func parsePoolID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("pool_id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid pool_id")
	}
	return id, nil
}

func decodePoolJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("request body must contain only one JSON object")
	}

	return nil
}

func writePoolError(w http.ResponseWriter, err error) {
	message := err.Error()
	lower := strings.ToLower(message)

	switch {
	case strings.Contains(lower, "resource pool not found"),
		strings.Contains(lower, "node not found"):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": message})

	case strings.Contains(lower, "already belongs"),
		strings.Contains(lower, "already exists"),
		strings.Contains(lower, "disabled"),
		strings.Contains(lower, "offline"),
		strings.Contains(lower, "not a member"),
		strings.Contains(lower, "not in this resource pool"):
		writeJSON(w, http.StatusConflict, map[string]string{"error": message})

	case strings.Contains(lower, "cannot be empty"),
		strings.Contains(lower, "cannot exceed"),
		strings.Contains(lower, "must contain"),
		strings.Contains(lower, "invalid"):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": message})

	default:
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "internal server error"})
	}
}

// POST /api/v1/pools
func (h *Handler) createPool(w http.ResponseWriter, r *http.Request) {
	var req createPoolRequest

	if err := decodePoolJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)

	if req.Name == "" || len(req.Name) > 64 {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "pool name must contain 1-64 characters"})
		return
	}

	pool, err := h.repository.CreatePool(r.Context(), req.Name, req.Description)
	if err != nil {
		writePoolError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"pool": pool})
}

// GET /api/v1/pools
func (h *Handler) listPools(w http.ResponseWriter, r *http.Request) {
	pools, err := h.repository.ListPools(r.Context())
	if err != nil {
		writePoolError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total": len(pools),
		"pools": pools,
	})
}

// GET /api/v1/pools/{pool_id}
func (h *Handler) getPool(w http.ResponseWriter, r *http.Request) {
	poolID, err := parsePoolID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	pool, err := h.repository.GetPool(r.Context(), poolID)
	if err != nil {
		writePoolError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"pool": pool})
}

// PUT /api/v1/pools/{pool_id}
func (h *Handler) updatePool(w http.ResponseWriter, r *http.Request) {
	poolID, err := parsePoolID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var req updatePoolRequest
	if err := decodePoolJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)

	if req.Name == "" || len(req.Name) > 64 {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "pool name must contain 1-64 characters"})
		return
	}

	if err := h.repository.UpdatePool(
		r.Context(), poolID, req.Name, req.Description,
	); err != nil {
		writePoolError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// DELETE /api/v1/pools/{pool_id}
func (h *Handler) deletePool(w http.ResponseWriter, r *http.Request) {
	poolID, err := parsePoolID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := h.repository.DeletePool(r.Context(), poolID); err != nil {
		writePoolError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// PATCH /api/v1/pools/{pool_id}/status
func (h *Handler) setPoolStatus(w http.ResponseWriter, r *http.Request) {
	poolID, err := parsePoolID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var req updatePoolStatusRequest
	if err := decodePoolJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if req.Status != "active" && req.Status != "disabled" {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "status must be active or disabled"})
		return
	}

	if err := h.repository.SetPoolStatus(r.Context(), poolID, req.Status); err != nil {
		writePoolError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": req.Status})
}

// POST /api/v1/pools/{pool_id}/nodes
func (h *Handler) addNodeToPool(w http.ResponseWriter, r *http.Request) {
	poolID, err := parsePoolID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var req addPoolNodeRequest
	if err := decodePoolJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	req.NodeID = strings.TrimSpace(req.NodeID)
	if req.NodeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id cannot be empty"})
		return
	}

	if err := h.repository.AddNodeToPool(
		r.Context(), poolID, req.NodeID,
	); err != nil {
		writePoolError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"status":  "added",
		"node_id": req.NodeID,
	})
}

// DELETE /api/v1/pools/{pool_id}/nodes/{node_id}
func (h *Handler) removeNodeFromPool(w http.ResponseWriter, r *http.Request) {
	poolID, err := parsePoolID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	nodeID := strings.TrimSpace(r.PathValue("node_id"))
	if nodeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id cannot be empty"})
		return
	}

	if err := h.repository.RemoveNodeFromPool(
		r.Context(), poolID, nodeID,
	); err != nil {
		writePoolError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "removed",
		"node_id": nodeID,
	})
}

// GET /api/v1/pools/{pool_id}/nodes
func (h *Handler) listPoolNodes(w http.ResponseWriter, r *http.Request) {
	poolID, err := parsePoolID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	nodes, err := h.repository.ListPoolNodes(r.Context(), poolID)
	if err != nil {
		writePoolError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"pool_id": poolID,
		"total":   len(nodes),
		"nodes":   nodes,
	})
}

// GET /api/v1/pools/{pool_id}/stats
func (h *Handler) getPoolStats(w http.ResponseWriter, r *http.Request) {
	poolID, err := parsePoolID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	stats, err := h.repository.GetPoolStats(r.Context(), poolID)
	if err != nil {
		writePoolError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// 确保资源池 Handler 使用的模型在编译期可见。
var _ = node.Pool{}
