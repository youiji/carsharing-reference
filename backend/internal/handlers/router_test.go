package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRouterNotFoundAndMethodNotAllowed(t *testing.T) {
	h := NewRouter(Deps{})
	tests := []struct {
		method, path string
		status       int
		allow        string
	}{
		{http.MethodGet, "/api/nope", 404, ""},
		{http.MethodDelete, "/api/health", 405, "GET"},
		{http.MethodPut, "/api/rentals/5", 405, "GET"},
		{http.MethodGet, "/api/rentals/5/start", 405, "POST"},
		{http.MethodDelete, "/api/me", 405, "GET"},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
		if rec.Code != tt.status || rec.Header().Get("Allow") != tt.allow {
			t.Errorf("%s %s: %d Allow=%q, want %d Allow=%q", tt.method, tt.path,
				rec.Code, rec.Header().Get("Allow"), tt.status, tt.allow)
		}
		if !strings.Contains(rec.Body.String(), `"error"`) {
			t.Errorf("%s %s: ответ не в едином формате ошибок: %s", tt.method, tt.path, rec.Body)
		}
	}
}
