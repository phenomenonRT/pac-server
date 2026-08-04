package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pac-server/internal/config"
)

func TestRoutesServeIndexPACAndStatic(t *testing.T) {
	app := New(config.Default(), t.TempDir()+"/config.json", slog.Default())
	handler := app.Routes()

	tests := []struct {
		name        string
		path        string
		contentType string
	}{
		{name: "index", path: "/", contentType: "text/html"},
		{name: "lists", path: "/lists", contentType: "text/html"},
		{name: "settings", path: "/settings", contentType: "text/html"},
		{name: "pac", path: "/proxy.pac", contentType: "application/x-ns-proxy-autoconfig"},
		{name: "named pac", path: "/pac/default.pac", contentType: "application/x-ns-proxy-autoconfig"},
		{name: "static", path: "/static/app.css", contentType: "text/css"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, tt.contentType) {
				t.Fatalf("Content-Type = %q, want prefix %q", got, tt.contentType)
			}
		})
	}
}
