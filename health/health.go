package health

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

type Response struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

type Handler struct {
	isHealthy atomic.Bool
}

func NewHandler() *Handler {
	h := &Handler{}
	h.isHealthy.Store(true)
	return h
}

func (h *Handler) SetHealthy(healthy bool) {
	h.isHealthy.Store(healthy)
}

func (h *Handler) IsHealthy() bool {
	return h.isHealthy.Load()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if !h.IsHealthy() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(Response{
			Status:    "DOWN",
			Timestamp: time.Now().UTC(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_ = json.NewEncoder(w).Encode(Response{
			Status:    "UP",
			Timestamp: time.Now().UTC(),
		})
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("/healthz", h)
	mux.Handle("/health", h)
}
