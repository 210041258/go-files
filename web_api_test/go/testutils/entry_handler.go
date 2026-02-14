// Package entryhandler provides HTTP handlers for managing key‑value entries
// in a storage backend that implements the store.Store interface.
package testutils

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

)

// EntryHandler implements HTTP handlers for a RESTful entry API.
type EntryHandler struct {
	store store.Store
}

// New creates a new EntryHandler with the given storage backend.
func New(s store.Store) *EntryHandler {
	return &EntryHandler{store: s}
}

// RegisterRoutes registers the entry API routes on the provided ServeMux.
func (h *EntryHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /entries", h.ListEntries)
	mux.HandleFunc("POST /entries", h.CreateEntry)
	mux.HandleFunc("GET /entries/{key}", h.GetEntry)
	mux.HandleFunc("PUT /entries/{key}", h.PutEntry)
	mux.HandleFunc("DELETE /entries/{key}", h.DeleteEntry)
}

// request and response structures
type (
	createEntryRequest struct {
		Key   string          `json:"key"`
		Value json.RawMessage `json:"value"`
		TTL   string          `json:"ttl,omitempty"` // e.g., "5s", "10m"
	}

	entryResponse struct {
		Key   string          `json:"key"`
		Value json.RawMessage `json:"value"`
	}

	errorResponse struct {
		Error string `json:"error"`
	}
)

// ListEntries responds with all keys matching the optional pattern query parameter.
// GET /entries?pattern=prefix*
func (h *EntryHandler) ListEntries(w http.ResponseWriter, r *http.Request) {
	pattern := r.URL.Query().Get("pattern")
	if pattern == "" {
		pattern = "%" // default: match all (depends on store implementation)
	}

	keys, err := h.store.List(r.Context(), pattern)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to list entries")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"keys": keys})
}

// CreateEntry creates a new entry. The request body must contain a JSON object
// with "key", "value", and optionally "ttl".
// POST /entries
func (h *EntryHandler) CreateEntry(w http.ResponseWriter, r *http.Request) {
	var req createEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	defer r.Body.Close()

	if req.Key == "" {
		h.writeError(w, http.StatusBadRequest, "key is required")
		return
	}
	if req.Value == nil {
		h.writeError(w, http.StatusBadRequest, "value is required")
		return
	}

	ttl, err := parseTTL(req.TTL)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid ttl format")
		return
	}

	if err := h.store.Set(r.Context(), req.Key, req.Value, ttl); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to store entry")
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entryResponse{
		Key:   req.Key,
		Value: req.Value,
	})
}

// GetEntry retrieves an entry by key.
// GET /entries/{key}
func (h *EntryHandler) GetEntry(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		h.writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	var value json.RawMessage
	if err := h.store.Get(r.Context(), key, &value); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.writeError(w, http.StatusNotFound, "entry not found")
		} else {
			h.writeError(w, http.StatusInternalServerError, "failed to get entry")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entryResponse{
		Key:   key,
		Value: value,
	})
}

// PutEntry creates or updates an entry. The key is taken from the URL path.
// The request body may contain a JSON object with "value" and optionally "ttl".
// PUT /entries/{key}
func (h *EntryHandler) PutEntry(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		h.writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	var req struct {
		Value json.RawMessage `json:"value"`
		TTL   string          `json:"ttl,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	defer r.Body.Close()

	if req.Value == nil {
		h.writeError(w, http.StatusBadRequest, "value is required")
		return
	}

	ttl, err := parseTTL(req.TTL)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid ttl format")
		return
	}

	if err := h.store.Set(r.Context(), key, req.Value, ttl); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to store entry")
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(entryResponse{
		Key:   key,
		Value: req.Value,
	})
}

// DeleteEntry removes an entry by key.
// DELETE /entries/{key}
func (h *EntryHandler) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		h.writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	if err := h.store.Delete(r.Context(), key); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to delete entry")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeError sends a JSON error response.
func (h *EntryHandler) writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{Error: msg})
}

// parseTTL converts a string like "10s", "5m", "2h" into a time.Duration.
// Returns 0 if the string is empty.
func parseTTL(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

// ----------------------------------------------------------------------
// Example usage:
//
// func main() {
//     ctx := context.Background()
//     store, _ := store.NewRedisStoreFromURL("redis://localhost:6379/0")
//     defer store.Close()
//
//     handler := entryhandler.New(store)
//     mux := http.NewServeMux()
//     handler.RegisterRoutes(mux)
//
//     http.ListenAndServe(":8080", mux)
// }
// ----------------------------------------------------------------------