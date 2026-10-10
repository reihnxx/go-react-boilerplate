package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var spec = SortSpec{Allowed: map[string]string{"name": "u.name", "created_at": "u.created_at"}, Default: "created_at", DefaultDesc: true}

func parse(t *testing.T, query string) (PageQuery, error) {
	t.Helper()
	return ParsePageQuery(httptest.NewRequest(http.MethodGet, "/x?"+query, nil), spec)
}

func TestParsePageQueryDefaults(t *testing.T) {
	q, err := parse(t, "")
	if err != nil {
		t.Fatal(err)
	}
	if q.Page != 1 || q.PerPage != DefaultPerPage || q.Offset() != 0 || q.OrderBy() != "u.created_at DESC" {
		t.Fatalf("unexpected defaults: %+v %s", q, q.OrderBy())
	}
}

func TestParsePageQueryValues(t *testing.T) {
	q, err := parse(t, "page=3&per_page=500&sort=name&order=asc&q=%20budi%20")
	if err != nil {
		t.Fatal(err)
	}
	if q.Page != 3 || q.PerPage != MaxPerPage || q.Offset() != 200 || q.Search != "budi" || q.OrderBy() != "u.name ASC" {
		t.Fatalf("unexpected: %+v %s", q, q.OrderBy())
	}
}

func TestParsePageQueryRejectsInvalid(t *testing.T) {
	for _, query := range []string{"page=0", "page=x", "per_page=-1", "sort=password_hash", "sort=name%3Bdrop", "order=sideways"} {
		if _, err := parse(t, query); err == nil {
			t.Errorf("%q: expected error", query)
		}
	}
}

func TestPageMeta(t *testing.T) {
	m := NewPageMeta(PageQuery{Page: 2, PerPage: 20}, 41)
	if m.TotalPages != 3 || m.Total != 41 {
		t.Fatalf("unexpected meta %+v", m)
	}
}

func TestLikePatternEscapes(t *testing.T) {
	if got := LikePattern(`50%_a\b`); got != `%50\%\_a\\b%` {
		t.Fatalf("LikePattern = %q", got)
	}
}

func TestWriteErrorHidesInternalErrors(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, httptest.NewRequest(http.MethodGet, "/", nil), errors.New("pq: password authentication failed for user"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "password authentication") {
		t.Fatal("internal error leaked to client")
	}
}

func TestWriteErrorAPIError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, httptest.NewRequest(http.MethodGet, "/", nil), TooManyRequests("wait", 1500*time.Millisecond))
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "2" {
		t.Fatalf("status=%d retry=%q", rec.Code, rec.Header().Get("Retry-After"))
	}
	var body struct {
		Error struct {
			Code   string            `json:"code"`
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	rec = httptest.NewRecorder()
	WriteError(rec, httptest.NewRequest(http.MethodGet, "/", nil), FieldError("email", "Email sudah digunakan."))
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 422 || body.Error.Code != CodeValidation || body.Error.Fields["email"] == "" {
		t.Fatalf("unexpected: %d %+v", rec.Code, body)
	}
}

func TestDecode(t *testing.T) {
	var v struct{ A string }
	if err := Decode(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"A":"x"}`)), &v); err != nil || v.A != "x" {
		t.Fatalf("decode: %v %+v", err, v)
	}
	for _, body := range []string{"", "{", "[1,2"} {
		var apiErr *Error
		err := Decode(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), &v)
		if !errors.As(err, &apiErr) || apiErr.Status != http.StatusBadRequest {
			t.Errorf("body %q: got %v", body, err)
		}
	}
}
