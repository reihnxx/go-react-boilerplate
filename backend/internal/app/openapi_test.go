package app

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/reihnxx/go-react-boilerplate/backend/api"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/config"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/mailer"
)

// TestOpenAPICoversAllRoutes keeps api/openapi.yaml in sync with the router.
func TestOpenAPICoversAllRoutes(t *testing.T) {
	cfg := config.Config{RequestTimeout: 1, AllowedOrigins: []string{"http://x"}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	a := New(cfg, nil, logger, mailer.New(config.MailConfig{Driver: "log"}, logger))

	spec := string(api.OpenAPI)
	routes := 0
	err := chi.Walk(a.Handler.(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		path, ok := strings.CutPrefix(route, "/api/v1")
		if !ok || path == "/openapi.yaml" || strings.HasSuffix(path, "/*") {
			return nil
		}
		routes++
		block := "\n  " + path + ":\n"
		i := strings.Index(spec, block)
		if i < 0 {
			t.Errorf("%s %s is not documented", method, path)
			return nil
		}
		// The method must appear before the next path entry.
		rest := spec[i+len(block)-1:]
		if j := strings.Index(rest, "\n  /"); j >= 0 {
			rest = rest[:j]
		}
		if !strings.Contains(rest, "\n    "+strings.ToLower(method)+":") {
			t.Errorf("%s %s: method not documented", method, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if routes < 20 {
		t.Fatalf("only %d routes found; walker broken?", routes)
	}
}
