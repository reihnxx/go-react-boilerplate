package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
)

func TestTrustedProxyIP(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{"x-real-ip used", map[string]string{"X-Real-IP": "203.0.113.9"}, "203.0.113.9"},
		{"ipv6", map[string]string{"X-Real-IP": "2001:db8::1"}, "2001:db8::1"},
		{"forwarded-for ignored", map[string]string{"X-Forwarded-For": "198.51.100.1"}, "192.0.2.10"},
		{"true-client-ip ignored", map[string]string{"True-Client-IP": "198.51.100.2"}, "192.0.2.10"},
		{"garbage ignored", map[string]string{"X-Real-IP": "1.2.3.4, 5.6.7.8"}, "192.0.2.10"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got string
			h := TrustedProxyIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = clientIP(r) }))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.0.2.10:5555"
			for k, v := range c.headers {
				req.Header.Set(k, v)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != c.want {
				t.Fatalf("client IP = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRequestIDHasNoHostname(t *testing.T) {
	var id string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { id = middleware.GetReqID(r.Context()) }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if len(id) != 16 || rec.Header().Get("X-Request-Id") != id {
		t.Fatalf("unexpected request id %q / header %q", id, rec.Header().Get("X-Request-Id"))
	}
}

func TestRecoverReturnsJSON500(t *testing.T) {
	h := Recover(discardLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError || rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("status %d, content-type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
}
