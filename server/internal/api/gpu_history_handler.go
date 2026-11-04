package api

import (
	"net/http"
	"strconv"
	"time"
)

// getGPUHistory 查询指定节点的 GPU 历史资源。
func (h *Handler) getGPUHistory(
	w http.ResponseWriter,
	r *http.Request,
) {
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

	query := r.URL.Query()

	// GPU 序号：不传表示查询节点全部 GPU。
	var gpuIndex *int

	if value := query.Get("gpu_index"); value != "" {
		index, err := strconv.Atoi(value)
		if err != nil || index < 0 {
			writeJSON(w, http.StatusBadRequest,
				map[string]string{"error": "gpu_index cannot be negative"})
			return
		}
		gpuIndex = &index
	}

	// 分页参数。
	limit := 20
	offset := 0

	if value := query.Get("limit"); value != "" {
		limit, err = strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 200 {
			writeJSON(w, http.StatusBadRequest,
				map[string]string{"error": "limit must be between 1 and 200"})
			return
		}
	}

	if value := query.Get("offset"); value != "" {
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

	if value := query.Get("from"); value != "" {
		parsed, parseErr := time.Parse(time.RFC3339, value)
		if parseErr != nil {
			writeJSON(w, http.StatusBadRequest,
				map[string]string{"error": "invalid from time"})
			return
		}
		from = &parsed
	}

	if value := query.Get("to"); value != "" {
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

	// 查询 GPU 历史。
	history, err := h.repository.GetGPUHistory(
		r.Context(),
		nodeID,
		gpuIndex,
		from,
		to,
		limit,
		offset,
	)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "failed to query GPU history"})
		return
	}

	writeJSON(w, http.StatusOK, history)
}
