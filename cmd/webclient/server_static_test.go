package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestSPAFallback verifies that non-API routes serve index.html.
func TestSPAFallback(t *testing.T) {
	// Write a temp index.html.
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html>SPA</html>"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := buildStaticHandler(dir)

	tests := []struct {
		path       string
		wantStatus int
	}{
		{"/", http.StatusOK},
		{"/characters", http.StatusOK},
		{"/game", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != tt.wantStatus {
				t.Errorf("path %s: status = %d, want %d", tt.path, rr.Code, tt.wantStatus)
			}
			if body := rr.Body.String(); len(body) == 0 {
				t.Errorf("path %s: expected non-empty body", tt.path)
			}
		})
	}
}
