package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type sampleOut struct {
	Answer string `json:"answer"`
}

func TestChatWithSchema_Success(t *testing.T) {
	var captured atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured.Store(string(body))
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("missing content-type header")
		}
		resp := map[string]interface{}{
			"message": map[string]interface{}{"role": "assistant", "content": `{"answer":"42"}`},
			"done":    true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	messages := []ChatMessage{{Role: "user", Content: "q?"}}
	schema := map[string]interface{}{"type": "object"}
	var out sampleOut
	if err := ChatWithSchema(context.Background(), srv.URL, "m1", messages, schema, &out); err != nil {
		t.Fatalf("ChatWithSchema: %v", err)
	}
	if out.Answer != "42" {
		t.Errorf("answer=%q", out.Answer)
	}
	raw, _ := captured.Load().(string)
	if !strings.Contains(raw, `"model":"m1"`) {
		t.Errorf("missing model in body: %s", raw)
	}
	if !strings.Contains(raw, `"stream":false`) {
		t.Errorf("stream=false missing: %s", raw)
	}
	if !strings.Contains(raw, `"format":`) {
		t.Errorf("format missing: %s", raw)
	}
	if !strings.Contains(raw, `"messages":`) {
		t.Errorf("messages missing: %s", raw)
	}
}

func TestChatWithSchema_4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "bad")
	}))
	defer srv.Close()
	var out sampleOut
	err := ChatWithSchema(context.Background(), srv.URL, "m", nil, map[string]any{}, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "status") {
		t.Errorf("expected status in error: %v", err)
	}
}

func TestChatWithSchema_5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	var out sampleOut
	err := ChatWithSchema(context.Background(), srv.URL, "m", nil, map[string]any{}, &out)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestChatWithSchema_MalformedResponseJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "definitely not json")
	}))
	defer srv.Close()
	var out sampleOut
	err := ChatWithSchema(context.Background(), srv.URL, "m", nil, map[string]any{}, &out)
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("expected decode in error: %v", err)
	}
}

func TestChatWithSchema_NonJSONContentField(t *testing.T) {
	// The outer envelope is valid but inner content is not JSON.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"message": map[string]interface{}{"content": "hello world"},
			"done":    true,
		})
	}))
	defer srv.Close()
	var out sampleOut
	err := ChatWithSchema(context.Background(), srv.URL, "m", nil, map[string]any{}, &out)
	if err == nil {
		t.Fatal("expected parse error on non-json content")
	}
	if !strings.Contains(err.Error(), "parse JSON output") {
		t.Errorf("expected parse JSON output in error: %v", err)
	}
}

func TestChatWithSchema_EmptyContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"message": map[string]interface{}{"content": ""},
			"done":    true,
		})
	}))
	defer srv.Close()
	var out sampleOut
	err := ChatWithSchema(context.Background(), srv.URL, "m", nil, map[string]any{}, &out)
	if err == nil {
		t.Fatal("expected empty-content error")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("want empty response error, got %v", err)
	}
}

func TestChatWithSchema_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(500 * time.Millisecond):
		case <-r.Context().Done():
			return
		}
		_, _ = io.WriteString(w, `{"message":{"content":"{}"},"done":true}`)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	var out sampleOut
	err := ChatWithSchema(ctx, srv.URL, "m", nil, map[string]any{}, &out)
	if err == nil {
		t.Fatal("expected error from cancelled ctx")
	}
}

func TestChatWithSchema_BadBaseURL(t *testing.T) {
	// A control-character URL fails url.Parse.
	var out sampleOut
	err := ChatWithSchema(context.Background(), "http://\x7f", "m", nil, map[string]any{}, &out)
	if err == nil {
		t.Fatal("expected URL parse error")
	}
}

func TestChatWithSchema_UnmarshalSchemaFails(t *testing.T) {
	// A schema containing a channel cannot be marshalled.
	var out sampleOut
	err := ChatWithSchema(context.Background(), "http://localhost", "m", nil, make(chan int), &out)
	if err == nil {
		t.Fatal("expected marshal error")
	}
}
