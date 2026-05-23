package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	serveMode  bool
	serverPort int
	globalBus  *eventBus
)

// busEvent is one SSE-shaped event ready to be flushed to clients.
type busEvent struct {
	Type string
	Data []byte
}

// eventBus is a tiny in-process pub/sub for slap events.
// Subscribers are non-blocking: if a subscriber's buffer fills up,
// events are dropped for that subscriber to avoid stalling the
// detection loop.
type eventBus struct {
	mu          sync.RWMutex
	subscribers map[chan busEvent]struct{}
}

func newEventBus() *eventBus {
	return &eventBus{subscribers: make(map[chan busEvent]struct{})}
}

func (eb *eventBus) Subscribe() (<-chan busEvent, func()) {
	ch := make(chan busEvent, 32)
	eb.mu.Lock()
	eb.subscribers[ch] = struct{}{}
	eb.mu.Unlock()

	return ch, func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()
		if _, ok := eb.subscribers[ch]; ok {
			delete(eb.subscribers, ch)
			close(ch)
		}
	}
}

func (eb *eventBus) Publish(eventType string, data []byte) {
	ev := busEvent{Type: eventType, Data: data}
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for ch := range eb.subscribers {
		select {
		case ch <- ev:
		default:
			// drop if subscriber is slow
		}
	}
}

// publishBus publishes a typed JSON event to the global event bus.
// Safe to call when serve mode is disabled (no-op).
func publishBus(eventType string, payload interface{}) {
	if globalBus == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	globalBus.Publish(eventType, data)
}

// applySettings updates runtime settings from optional pointers.
// nil means "do not change". Out-of-range values are silently ignored
// to match the existing stdio set semantics.
func applySettings(amp *float64, cooldown *int, speed *float64, vs *bool) {
	if amp != nil && *amp > 0 && *amp <= 1 {
		minAmplitude = *amp
	}
	if cooldown != nil && *cooldown > 0 {
		cooldownMs = *cooldown
	}
	if speed != nil && *speed > 0 {
		speedRatio = *speed
	}
	if vs != nil {
		volumeScaling = *vs
	}
}

// currentSettings returns a snapshot of the mutable runtime settings.
func currentSettings() map[string]interface{} {
	return map[string]interface{}{
		"amplitude":      minAmplitude,
		"cooldown":       cooldownMs,
		"speed":          speedRatio,
		"volume_scaling": volumeScaling,
	}
}

// currentStatus returns a full snapshot including read-only metadata.
func currentStatus() map[string]interface{} {
	pausedMu.RLock()
	isPaused := paused
	pausedMu.RUnlock()
	s := currentSettings()
	s["active_pack_id"] = activePackID
	s["mode"] = currentMode
	s["preset"] = currentPreset
	s["paused"] = isPaused
	s["version"] = version
	return s
}

func setPaused(p bool) {
	pausedMu.Lock()
	paused = p
	pausedMu.Unlock()
}

// ----- HTTP layer -----

// corsMiddleware wraps a handler with permissive CORS headers
// suitable for local-only Next.js dev servers (e.g. localhost:3000).
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func writeJSONResponse(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErrorJSON(w http.ResponseWriter, status int, msg string) {
	writeJSONResponse(w, status, map[string]string{"error": msg})
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	s := currentStatus()
	s["status"] = "ok"
	writeJSONResponse(w, http.StatusOK, s)
}

func handlePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	setPaused(true)
	writeJSONResponse(w, http.StatusOK, map[string]string{"status": "paused"})
}

func handleResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	setPaused(false)
	writeJSONResponse(w, http.StatusOK, map[string]string{"status": "resumed"})
}

// setRequest mirrors the stdin set command but uses pointers
// so the caller can update individual fields.
type setRequest struct {
	Amplitude     *float64 `json:"amplitude,omitempty"`
	Cooldown      *int     `json:"cooldown,omitempty"`
	Speed         *float64 `json:"speed,omitempty"`
	VolumeScaling *bool    `json:"volume_scaling,omitempty"`
}

func handleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}
	defer r.Body.Close()

	var req setRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "invalid json: "+err.Error())
			return
		}
	}
	applySettings(req.Amplitude, req.Cooldown, req.Speed, req.VolumeScaling)

	resp := currentSettings()
	resp["status"] = "settings_updated"
	writeJSONResponse(w, http.StatusOK, resp)
}

func handlePacks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"packs":          listPacks(),
		"active_pack_id": activePackID,
	})
}

type activatePackRequest struct {
	ID string `json:"id"`
}

func handleActivatePack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}
	defer r.Body.Close()

	var req activatePackRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.ID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "id is required")
		return
	}

	info, err := activatePack(req.ID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"status":         "pack_activated",
		"active_pack_id": req.ID,
		"pack":           info,
	})
}

func handleImportPack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPackTotalBytes+(10<<20))
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "parse multipart form: "+err.Error())
		return
	}

	name := r.FormValue("name")
	files := r.MultipartForm.File["files"]
	info, err := importUserPack(name, files)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"status": "pack_imported",
		"pack":   info,
	})
}

type deletePackRequest struct {
	ID string `json:"id"`
}

func handleDeletePack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeErrorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}
	defer r.Body.Close()

	var req deletePackRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.ID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "id is required")
		return
	}

	info, err := deleteUserPack(req.ID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"status":          "pack_deleted",
		"deleted_pack_id": req.ID,
		"active_pack_id":  activePackID,
		"pack":            info,
	})
}

// handleEvents streams ready / slap / bye events as Server-Sent Events.
// Clients should use the browser's EventSource API.
func handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if globalBus == nil {
		writeErrorJSON(w, http.StatusServiceUnavailable, "event bus unavailable")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErrorJSON(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ch, unsubscribe := globalBus.Subscribe()
	defer unsubscribe()

	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			// SSE comment line so proxies and load balancers don't drop the connection.
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, ev.Data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// runHTTPServer starts the HTTP listener until ctx is canceled.
// Listens only on 127.0.0.1 to avoid exposing the API to the network.
func runHTTPServer(ctx context.Context, port int) error {
	if globalBus == nil {
		globalBus = newEventBus()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", corsMiddleware(handleStatus))
	mux.HandleFunc("/api/pause", corsMiddleware(handlePause))
	mux.HandleFunc("/api/resume", corsMiddleware(handleResume))
	mux.HandleFunc("/api/set", corsMiddleware(handleSet))
	mux.HandleFunc("/api/packs", corsMiddleware(handlePacks))
	mux.HandleFunc("/api/packs/activate", corsMiddleware(handleActivatePack))
	mux.HandleFunc("/api/packs/import", corsMiddleware(handleImportPack))
	mux.HandleFunc("/api/packs/delete", corsMiddleware(handleDeletePack))
	mux.HandleFunc("/api/events", corsMiddleware(handleEvents))

	addr := "127.0.0.1:" + strconv.Itoa(port)
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	fmt.Fprintf(os.Stderr, "pafu: http server listening on http://%s\n", addr)

	errCh := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		return err
	}
}
