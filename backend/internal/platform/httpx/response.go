package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

type envelope struct {
	Data any       `json:"data"`
	Meta *PageMeta `json:"meta,omitempty"`
}

type errorBody struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

// JSON writes v as a JSON response with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "err", err)
	}
}

func OK(w http.ResponseWriter, data any)      { JSON(w, http.StatusOK, envelope{Data: data}) }
func Created(w http.ResponseWriter, data any) { JSON(w, http.StatusCreated, envelope{Data: data}) }
func NoContent(w http.ResponseWriter)         { w.WriteHeader(http.StatusNoContent) }

// Paginated writes a list response with pagination metadata.
func Paginated(w http.ResponseWriter, data any, meta PageMeta) {
	JSON(w, http.StatusOK, envelope{Data: data, Meta: &meta})
}

// HandlerFunc is an http handler that returns an error instead of writing it.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// Handle adapts a HandlerFunc, rendering any returned error via WriteError.
func Handle(fn HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			WriteError(w, r, err)
		}
	}
}

// WriteError renders err using the error envelope. Unknown errors are logged
// and replaced with a generic 500 so internals never reach the client.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	reqID := middleware.GetReqID(r.Context())

	var apiErr *Error
	if !errors.As(err, &apiErr) {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			slog.WarnContext(r.Context(), "request aborted", "err", err, "request_id", reqID, "path", r.URL.Path)
			apiErr = newError(http.StatusServiceUnavailable, CodeInternal, "Permintaan melebihi batas waktu. Coba lagi.")
		} else {
			slog.ErrorContext(r.Context(), "unhandled error", "err", err, "request_id", reqID,
				"method", r.Method, "path", r.URL.Path)
			apiErr = newError(http.StatusInternalServerError, CodeInternal, "Terjadi kesalahan pada server.")
		}
	}

	if apiErr.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(apiErr.RetryAfter.Seconds()))))
	}
	JSON(w, apiErr.Status, map[string]errorBody{"error": {
		Code:      apiErr.Code,
		Message:   apiErr.Message,
		Fields:    apiErr.Fields,
		RequestID: reqID,
	}})
}

// Decode parses a JSON request body into dst.
func Decode(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr):
			return newError(http.StatusRequestEntityTooLarge, CodePayloadTooLarge, "Ukuran data terlalu besar.")
		case errors.Is(err, io.EOF):
			return BadRequest("Body permintaan kosong.")
		default:
			return BadRequest("Format JSON tidak valid.")
		}
	}
	return nil
}

// PathUUID reads a UUID URL parameter; malformed IDs are reported as 404.
func PathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, NotFound("")
	}
	return id, nil
}
