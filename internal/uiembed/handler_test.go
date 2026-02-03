//go:build uiembed

package uiembed

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHandlerServesIndex(t *testing.T) {
	h, err := NewHandler("/ui")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, resp.Code)
	}
	if !bytes.Contains(resp.Body.Bytes(), []byte("id=\"app\"")) {
		t.Fatalf("expected index.html content")
	}
}

func TestHandlerSpaFallback(t *testing.T) {
	h, err := NewHandler("/ui")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/trace/abc", nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, resp.Code)
	}
	if !bytes.Contains(resp.Body.Bytes(), []byte("id=\"app\"")) {
		t.Fatalf("expected index.html content")
	}
}

func TestHandlerServesAsset(t *testing.T) {
	assetPath := findAsset(t)
	h, err := NewHandler("/ui")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, assetPath, nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, resp.Code)
	}
	if resp.Body.Len() == 0 {
		t.Fatalf("expected asset body")
	}
}

func findAsset(t *testing.T) string {
	t.Helper()
	entries, err := os.ReadDir("dist/assets")
	if err != nil {
		t.Fatalf("read dist/assets: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		return "/assets/" + entry.Name()
	}
	t.Fatalf("no assets found in dist/assets")
	return ""
}
