// Package validate collects per-field validation messages.
package validate

import (
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/httpx"
)

// Errors maps a JSON field name to its first validation message.
type Errors map[string]string

func New() Errors { return Errors{} }

// Add records msg for field unless the field already has a message.
func (e Errors) Add(field, msg string) {
	if _, ok := e[field]; !ok {
		e[field] = msg
	}
}

func (e Errors) Has(field string) bool { _, ok := e[field]; return ok }

// Check adds msg when ok is false.
func (e Errors) Check(ok bool, field, msg string) {
	if !ok {
		e.Add(field, msg)
	}
}

func (e Errors) Required(field, value, label string) {
	e.Check(strings.TrimSpace(value) != "", field, label+" wajib diisi.")
}

func (e Errors) MaxLength(field, value string, max int, label string) {
	e.Check(utf8.RuneCountInString(value) <= max, field, label+" maksimal "+strconv.Itoa(max)+" karakter.")
}

func (e Errors) MinLength(field, value string, min int, label string) {
	e.Check(utf8.RuneCountInString(value) >= min, field, label+" minimal "+strconv.Itoa(min)+" karakter.")
}

func (e Errors) Match(field, value string, re *regexp.Regexp, msg string) {
	e.Check(re.MatchString(value), field, msg)
}

// Email validates a bare address (no display name).
func (e Errors) Email(field, value string) {
	if value == "" {
		return
	}
	addr, err := mail.ParseAddress(value)
	e.Check(err == nil && addr.Address == value && len(value) <= 255, field, "Format email tidak valid.")
}

// Err returns a 422 API error when any message was recorded.
func (e Errors) Err() error {
	if len(e) == 0 {
		return nil
	}
	return httpx.Validation(e)
}
