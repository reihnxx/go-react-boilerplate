package validate

import (
	"errors"
	"regexp"
	"testing"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/httpx"
)

func TestErrors(t *testing.T) {
	e := New()
	e.Required("name", "  ", "Nama")
	e.Required("name", "", "Nama lain") // first message wins
	e.MaxLength("bio", "ééé", 2, "Bio") // counts runes, not bytes
	e.MinLength("code", "ab", 3, "Kode")
	e.Match("slug", "Bad Slug", regexp.MustCompile(`^[a-z_]+$`), "Slug tidak valid.")
	e.Email("email", "Budi <budi@example.com>")
	e.Email("empty", "")
	e.Check(true, "ok", "never")

	want := map[string]string{
		"name":  "Nama wajib diisi.",
		"bio":   "Bio maksimal 2 karakter.",
		"code":  "Kode minimal 3 karakter.",
		"slug":  "Slug tidak valid.",
		"email": "Format email tidak valid.",
	}
	for k, v := range want {
		if e[k] != v {
			t.Errorf("%s = %q, want %q", k, e[k], v)
		}
	}
	if e.Has("empty") || e.Has("ok") {
		t.Error("unexpected messages")
	}

	var apiErr *httpx.Error
	if err := e.Err(); !errors.As(err, &apiErr) || apiErr.Status != 422 || len(apiErr.Fields) != len(want) {
		t.Fatalf("Err() = %v", err)
	}
	if New().Err() != nil {
		t.Fatal("empty Errors must return nil")
	}
}

func TestEmail(t *testing.T) {
	valid := []string{"a@example.com", "first.last+tag@sub.example.co.id"}
	invalid := []string{"plain", "a@", "@example.com", "a b@example.com"}
	for _, v := range valid {
		e := New()
		e.Email("email", v)
		if e.Has("email") {
			t.Errorf("%q should be valid", v)
		}
	}
	for _, v := range invalid {
		e := New()
		e.Email("email", v)
		if !e.Has("email") {
			t.Errorf("%q should be invalid", v)
		}
	}
}
