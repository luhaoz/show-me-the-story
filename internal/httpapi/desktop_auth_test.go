package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDesktopAuthMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := desktopAuthMiddleware("secret", next)
	tests := []struct {
		name   string
		path   string
		header string
		want   int
	}{
		{name: "missing", path: "/api/status", want: http.StatusUnauthorized},
		{name: "header", path: "/api/status", header: "secret", want: http.StatusNoContent},
		{name: "sse rejects header", path: "/api/events", header: "secret", want: http.StatusUnauthorized},
		{name: "sse query", path: "/api/events?desktop_token=secret", want: http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.header != "" {
				req.Header.Set("X-Desktop-Token", tt.header)
			}
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
			if resp.Code != tt.want {
				t.Fatalf("got %d want %d", resp.Code, tt.want)
			}
		})
	}
}
