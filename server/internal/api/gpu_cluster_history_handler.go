package api

import (
	"net/http"
	"time"
)

func (h *Handler) getGPUClusterHistory(
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

	to := time.Now().UTC()
	from := to.Add(-duration)

	history, err := h.repository.GetGPUClusterHistory(
		r.Context(),
		from,
		to,
		interval,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "failed to query GPU cluster history"})
		return
	}

	history.Range = rangeValue
	writeJSON(w, http.StatusOK, history)
}
