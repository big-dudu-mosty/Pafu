package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// resetHTTPGlobals resets per-test globals shared with stdin tests.
func resetHTTPGlobals() {
	pausedMu.Lock()
	paused = false
	pausedMu.Unlock()
	minAmplitude = 0.05
	cooldownMs = 750
	speedRatio = 1.0
	volumeScaling = false
	stdioMode = false
	currentMode = "test"
	currentPreset = "default"
	globalBus = newEventBus()
	packRegistry = map[string]*soundPack{
		"pain": {name: "pain", mode: modeRandom, files: []string{"pain.mp3"}},
		"halo": {name: "halo", mode: modeRandom, files: []string{"halo.mp3"}},
	}
	activePackID = "pain"
	activeRuntime = nil
	setActivePack("pain", packRegistry["pain"], time.Duration(cooldownMs)*time.Millisecond)
}

func withTempAppSupport(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	old := appSupportDirOverride
	appSupportDirOverride = dir
	t.Cleanup(func() {
		appSupportDirOverride = old
	})
}

func TestHTTPStatus(t *testing.T) {
	resetHTTPGlobals()
	minAmplitude = 0.12
	cooldownMs = 600

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	corsMiddleware(handleStatus)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}
	if body["mode"] != "pain" {
		t.Errorf("expected mode=pain, got %v", body["mode"])
	}
	if body["active_pack_id"] != "pain" {
		t.Errorf("expected active_pack_id=pain, got %v", body["active_pack_id"])
	}
	if body["amplitude"].(float64) != 0.12 {
		t.Errorf("expected amplitude=0.12, got %v", body["amplitude"])
	}
	if body["cooldown"].(float64) != 600 {
		t.Errorf("expected cooldown=600, got %v", body["cooldown"])
	}
	if body["paused"].(bool) != false {
		t.Errorf("expected paused=false, got %v", body["paused"])
	}

	// CORS header should be present.
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected CORS header *, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestHTTPPauseResume(t *testing.T) {
	resetHTTPGlobals()

	// Pause.
	req := httptest.NewRequest(http.MethodPost, "/api/pause", nil)
	rec := httptest.NewRecorder()
	corsMiddleware(handlePause)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("pause expected 200, got %d", rec.Code)
	}
	pausedMu.RLock()
	if !paused {
		t.Error("expected paused=true after /api/pause")
	}
	pausedMu.RUnlock()

	// Resume.
	req = httptest.NewRequest(http.MethodPost, "/api/resume", nil)
	rec = httptest.NewRecorder()
	corsMiddleware(handleResume)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("resume expected 200, got %d", rec.Code)
	}
	pausedMu.RLock()
	if paused {
		t.Error("expected paused=false after /api/resume")
	}
	pausedMu.RUnlock()
}

func TestHTTPSetPartialUpdate(t *testing.T) {
	resetHTTPGlobals()

	body := strings.NewReader(`{"amplitude":0.18,"volume_scaling":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/set", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	corsMiddleware(handleSet)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if minAmplitude != 0.18 {
		t.Errorf("expected minAmplitude=0.18, got %v", minAmplitude)
	}
	if !volumeScaling {
		t.Error("expected volumeScaling=true")
	}
	// cooldown and speed were not in the request, must be unchanged.
	if cooldownMs != 750 {
		t.Errorf("expected cooldownMs unchanged 750, got %d", cooldownMs)
	}
	if speedRatio != 1.0 {
		t.Errorf("expected speed unchanged 1.0, got %v", speedRatio)
	}
}

func TestHTTPSetExplicitFalseVolumeScaling(t *testing.T) {
	resetHTTPGlobals()
	volumeScaling = true

	body := strings.NewReader(`{"volume_scaling":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/set", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	corsMiddleware(handleSet)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if volumeScaling {
		t.Error("expected volumeScaling=false after explicit set")
	}
}

func TestHTTPSetIgnoresOutOfRange(t *testing.T) {
	resetHTTPGlobals()
	original := minAmplitude

	body := strings.NewReader(`{"amplitude":2.0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/set", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	corsMiddleware(handleSet)(rec, req)

	if minAmplitude != original {
		t.Errorf("amplitude must not change for out-of-range, got %v", minAmplitude)
	}
}

func TestHTTPSetInvalidJSON(t *testing.T) {
	resetHTTPGlobals()

	body := strings.NewReader(`{not json`)
	req := httptest.NewRequest(http.MethodPost, "/api/set", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	corsMiddleware(handleSet)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", rec.Code)
	}
}

func TestHTTPPacksList(t *testing.T) {
	resetHTTPGlobals()

	req := httptest.NewRequest(http.MethodGet, "/api/packs", nil)
	rec := httptest.NewRecorder()
	corsMiddleware(handlePacks)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body struct {
		ActivePackID string     `json:"active_pack_id"`
		Packs        []packInfo `json:"packs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if body.ActivePackID != "pain" {
		t.Errorf("expected active pack pain, got %q", body.ActivePackID)
	}
	if len(body.Packs) != 2 {
		t.Fatalf("expected 2 packs, got %d", len(body.Packs))
	}
	if !body.Packs[1].Active {
		t.Errorf("expected pain pack to be active after deterministic sort, got %+v", body.Packs)
	}
}

func TestHTTPActivatePack(t *testing.T) {
	resetHTTPGlobals()

	body := strings.NewReader(`{"id":"halo"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/packs/activate", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	corsMiddleware(handleActivatePack)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if activePackID != "halo" {
		t.Errorf("expected activePackID=halo, got %q", activePackID)
	}
	if currentMode != "halo" {
		t.Errorf("expected currentMode=halo, got %q", currentMode)
	}

	var bodyResp struct {
		Status       string   `json:"status"`
		ActivePackID string   `json:"active_pack_id"`
		Pack         packInfo `json:"pack"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &bodyResp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if bodyResp.Status != "pack_activated" {
		t.Errorf("expected pack_activated, got %q", bodyResp.Status)
	}
	if bodyResp.ActivePackID != "halo" {
		t.Errorf("expected active pack halo in response, got %q", bodyResp.ActivePackID)
	}
	if bodyResp.Pack.ID != "halo" || !bodyResp.Pack.Active {
		t.Errorf("unexpected pack response: %+v", bodyResp.Pack)
	}
}

func TestHTTPActivatePackNotFound(t *testing.T) {
	resetHTTPGlobals()

	body := strings.NewReader(`{"id":"missing"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/packs/activate", body)
	rec := httptest.NewRecorder()
	corsMiddleware(handleActivatePack)(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestActivatePackPublishesEvent(t *testing.T) {
	resetHTTPGlobals()
	ch, unsub := globalBus.Subscribe()
	defer unsub()

	if _, err := activatePack("halo"); err != nil {
		t.Fatalf("activate pack: %v", err)
	}

	select {
	case ev := <-ch:
		if ev.Type != "pack-changed" {
			t.Fatalf("expected pack-changed event, got %q", ev.Type)
		}
		var payload struct {
			ActivePackID string   `json:"active_pack_id"`
			Pack         packInfo `json:"pack"`
		}
		if err := json.Unmarshal(ev.Data, &payload); err != nil {
			t.Fatalf("invalid event JSON: %v", err)
		}
		if payload.ActivePackID != "halo" {
			t.Errorf("expected halo event, got %q", payload.ActivePackID)
		}
	case <-time.After(time.Second):
		t.Fatal("pack-changed event was not received")
	}
}

func TestHTTPImportPack(t *testing.T) {
	resetHTTPGlobals()
	withTempAppSupport(t)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("name", "My Funny Pack"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	part, err := writer.CreateFormFile("files", "hello.mp3")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("fake mp3 data")); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/packs/import", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	corsMiddleware(handleImportPack)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Status string   `json:"status"`
		Pack   packInfo `json:"pack"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Status != "pack_imported" {
		t.Errorf("expected pack_imported, got %q", resp.Status)
	}
	if resp.Pack.ID == "" || resp.Pack.Name != "My Funny Pack" {
		t.Errorf("unexpected pack info: %+v", resp.Pack)
	}
	if resp.Pack.Source != "user" || resp.Pack.BuiltIn {
		t.Errorf("expected user non-builtin pack, got %+v", resp.Pack)
	}

	packRegistryMu.RLock()
	registered := packRegistry[resp.Pack.ID]
	packRegistryMu.RUnlock()
	if registered == nil {
		t.Fatalf("imported pack was not registered")
	}
	if len(registered.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(registered.files))
	}
	if _, err := os.Stat(registered.files[0]); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}
	if _, err := os.Stat(registered.dir + "/pack.json"); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
}

func TestHTTPImportPackRejectsNonMP3(t *testing.T) {
	resetHTTPGlobals()
	withTempAppSupport(t)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", "Bad Pack")
	part, err := writer.CreateFormFile("files", "bad.wav")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte("not mp3"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/packs/import", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	corsMiddleware(handleImportPack)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHTTPDeleteUserPack(t *testing.T) {
	resetHTTPGlobals()
	withTempAppSupport(t)

	dir := t.TempDir()
	file := dir + "/one.mp3"
	if err := os.WriteFile(file, []byte("fake"), 0o644); err != nil {
		t.Fatalf("write temp mp3: %v", err)
	}
	pack := &soundPack{
		name:        "custom",
		displayName: "Uploaded",
		dir:         dir,
		mode:        modeRandom,
		files:       []string{file},
		custom:      true,
		source:      "user",
		userManaged: true,
	}
	registerPack("uploaded", pack)
	setActivePack("uploaded", pack, time.Duration(cooldownMs)*time.Millisecond)

	req := httptest.NewRequest(http.MethodDelete, "/api/packs/delete", strings.NewReader(`{"id":"uploaded"}`))
	rec := httptest.NewRecorder()
	corsMiddleware(handleDeletePack)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if activePackID != "pain" {
		t.Errorf("expected active pack to fallback to pain, got %q", activePackID)
	}
	packRegistryMu.RLock()
	_, exists := packRegistry["uploaded"]
	packRegistryMu.RUnlock()
	if exists {
		t.Fatal("expected uploaded pack to be unregistered")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected pack directory removed, stat err=%v", err)
	}
}

func TestHTTPDeleteBuiltinPackRejected(t *testing.T) {
	resetHTTPGlobals()

	req := httptest.NewRequest(http.MethodDelete, "/api/packs/delete", strings.NewReader(`{"id":"pain"}`))
	rec := httptest.NewRecorder()
	corsMiddleware(handleDeletePack)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHTTPMethodNotAllowed(t *testing.T) {
	resetHTTPGlobals()

	req := httptest.NewRequest(http.MethodGet, "/api/pause", nil)
	rec := httptest.NewRecorder()
	corsMiddleware(handlePause)(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHTTPCORSPreflight(t *testing.T) {
	resetHTTPGlobals()

	req := httptest.NewRequest(http.MethodOptions, "/api/pause", nil)
	rec := httptest.NewRecorder()
	corsMiddleware(handlePause)(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS header on OPTIONS")
	}
}

func TestEventBusPublishSubscribe(t *testing.T) {
	resetHTTPGlobals()
	bus := newEventBus()

	ch, unsub := bus.Subscribe()
	defer unsub()

	bus.Publish("slap", []byte(`{"foo":1}`))

	select {
	case ev := <-ch:
		if ev.Type != "slap" {
			t.Errorf("expected type=slap, got %q", ev.Type)
		}
		if string(ev.Data) != `{"foo":1}` {
			t.Errorf("unexpected data %q", ev.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("event was not received")
	}
}

func TestEventBusDropsOnSlowSubscriber(t *testing.T) {
	bus := newEventBus()
	_, unsub := bus.Subscribe()
	defer unsub()

	// Publish many more than the buffer size; must not block.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			bus.Publish("slap", []byte(`{}`))
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publisher blocked on slow subscriber")
	}
}

func TestEventBusUnsubscribe(t *testing.T) {
	bus := newEventBus()
	ch, unsub := bus.Subscribe()
	unsub()

	bus.Publish("slap", []byte(`{}`))

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel closed after unsubscribe")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("channel was not closed after unsubscribe")
	}
}

func TestSSEStreamReceivesEvents(t *testing.T) {
	resetHTTPGlobals()

	server := httptest.NewServer(corsMiddleware(handleEvents))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected SSE content-type, got %q", resp.Header.Get("Content-Type"))
	}

	// Publish a slap event from another goroutine and read it back.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		publishBus("slap", map[string]interface{}{"slapNumber": 42, "amplitude": 0.5})
	}()

	got := readSSEEvent(t, resp.Body, 2*time.Second)
	wg.Wait()

	if got.event != "slap" {
		t.Errorf("expected event=slap, got %q", got.event)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(got.data), &payload); err != nil {
		t.Fatalf("invalid SSE data JSON: %v", err)
	}
	if payload["slapNumber"].(float64) != 42 {
		t.Errorf("expected slapNumber=42, got %v", payload["slapNumber"])
	}
}

type sseFrame struct {
	event string
	data  string
}

// readSSEEvent reads a single SSE frame (event/data lines) from r.
// Skips comment lines (": keepalive"). Errors out on timeout.
func readSSEEvent(t *testing.T, r io.Reader, timeout time.Duration) sseFrame {
	t.Helper()
	type result struct {
		f   sseFrame
		err error
	}
	done := make(chan result, 1)
	go func() {
		scanner := bufio.NewScanner(r)
		buf := make([]byte, 0, 1024)
		scanner.Buffer(buf, 1<<20)

		var frame sseFrame
		var dataLines []string
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				if frame.event != "" || len(dataLines) > 0 {
					frame.data = strings.Join(dataLines, "\n")
					done <- result{f: frame}
					return
				}
				continue
			}
			if strings.HasPrefix(line, ":") {
				continue
			}
			if strings.HasPrefix(line, "event: ") {
				frame.event = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
			}
		}
		done <- result{err: bytes.ErrTooLarge}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("read SSE: %v", r.err)
		}
		return r.f
	case <-time.After(timeout):
		t.Fatal("timeout waiting for SSE event")
		return sseFrame{}
	}
}
