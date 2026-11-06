package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"gpu-pool/server/internal/node"
)

func (h *Handler) getGPUCompareHistory(
	w http.ResponseWriter,
	r *http.Request,
) {
	if h == nil || h.repository == nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "repository unavailable"})
		return
	}

	rangeValue := r.URL.Query().Get("range")
	if rangeValue == "" {
		rangeValue = "1h"
	}

	var duration time.Duration
	var interval int

	switch rangeValue {
	case "1h":
		duration = time.Hour
		interval = 10
	case "6h":
		duration = 6 * time.Hour
		interval = 60
	case "24h":
		duration = 24 * time.Hour
		interval = 300
	default:
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "range must be 1h, 6h or 24h"})
		return
	}

	gpuValues := r.URL.Query()["gpu"]

	if len(gpuValues) == 0 || len(gpuValues) > 8 {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "select between 1 and 8 GPUs"})
		return
	}

	selected := make([]node.GPUCompareDevice, 0, len(gpuValues))
	seen := make(map[string]bool)

	for _, value := range gpuValues {
		nodeID, indexText, ok := strings.Cut(value, ":")
		if !ok || nodeID == "" {
			writeJSON(w, http.StatusBadRequest,
				map[string]string{"error": "invalid gpu parameter"})
			return
		}

		gpuIndex, err := strconv.Atoi(indexText)
		if err != nil || gpuIndex < 0 {
			writeJSON(w, http.StatusBadRequest,
				map[string]string{"error": "invalid gpu index"})
			return
		}

		key := nodeID + ":" + strconv.Itoa(gpuIndex)
		if seen[key] {
			continue
		}
		seen[key] = true

		selected = append(selected, node.GPUCompareDevice{
			NodeID:   nodeID,
			GPUIndex: gpuIndex,
		})
	}

	to := time.Now().UTC()
	from := to.Add(-duration)

	result, err := h.repository.GetGPUCompareHistory(
		r.Context(),
		selected,
		from,
		to,
		interval,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "failed to query GPU comparison"})
		return
	}

	result.Range = rangeValue
	writeJSON(w, http.StatusOK, result)
}
