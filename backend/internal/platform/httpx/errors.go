// Package httpx contains the JSON response envelope, typed API errors,
// request decoding and pagination helpers shared by all handlers.
package httpx

import (
	"net/http"
	"time"
)

// Error codes returned in the "error.code" field.
const (
	CodeBadRequest            = "BAD_REQUEST"
	CodeUnauthenticated       = "UNAUTHENTICATED"
	CodeInvalidCredentials    = "INVALID_CREDENTIALS"
	CodeForbidden             = "FORBIDDEN"
	CodeCSRFFailed            = "CSRF_FAILED"
	CodeRoleSelectionRequired = "ROLE_SELECTION_REQUIRED"
	CodeNotFound              = "NOT_FOUND"
	CodeMethodNotAllowed      = "METHOD_NOT_ALLOWED"
	CodeConflict              = "CONFLICT"
	CodePayloadTooLarge       = "PAYLOAD_TOO_LARGE"
	CodeValidation            = "VALIDATION_ERROR"
	CodeTooManyRequests       = "TOO_MANY_REQUESTS"
	CodeInternal              = "INTERNAL_ERROR"
)

// Error is an error that maps directly to an HTTP response. Anything else
// returned by a handler becomes a 500 with a generic message.
type Error struct {
	Status     int
	Code       string
	Message    string
	Fields     map[string]string
	RetryAfter time.Duration
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func newError(status int, code, msg string) *Error {
	return &Error{Status: status, Code: code, Message: msg}
}

func BadRequest(msg string) *Error { return newError(http.StatusBadRequest, CodeBadRequest, msg) }

func Unauthenticated() *Error {
	return newError(http.StatusUnauthorized, CodeUnauthenticated, "Sesi tidak valid atau sudah berakhir. Silakan login kembali.")
}

func InvalidCredentials(msg string) *Error {
	return newError(http.StatusUnauthorized, CodeInvalidCredentials, msg)
}

func Forbidden(msg string) *Error {
	if msg == "" {
		msg = "Anda tidak memiliki akses untuk melakukan aksi ini."
	}
	return newError(http.StatusForbidden, CodeForbidden, msg)
}

func CSRFFailed() *Error {
	return newError(http.StatusForbidden, CodeCSRFFailed, "Token keamanan tidak valid. Muat ulang halaman lalu coba lagi.")
}

func RoleSelectionRequired() *Error {
	return newError(http.StatusForbidden, CodeRoleSelectionRequired, "Pilih otoritas terlebih dahulu.")
}

func NotFound(msg string) *Error {
	if msg == "" {
		msg = "Data tidak ditemukan."
	}
	return newError(http.StatusNotFound, CodeNotFound, msg)
}

func Conflict(msg string) *Error { return newError(http.StatusConflict, CodeConflict, msg) }

// Validation returns a 422 with per-field messages.
func Validation(fields map[string]string) *Error {
	e := newError(http.StatusUnprocessableEntity, CodeValidation, "Data yang dikirim tidak valid.")
	e.Fields = fields
	return e
}

// FieldError is a shortcut for a single-field validation error.
func FieldError(field, msg string) *Error { return Validation(map[string]string{field: msg}) }

func TooManyRequests(msg string, retryAfter time.Duration) *Error {
	e := newError(http.StatusTooManyRequests, CodeTooManyRequests, msg)
	e.RetryAfter = retryAfter
	return e
}
