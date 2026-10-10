package httpx

import (
	"net/http"
	"strconv"
	"strings"
)

const (
	DefaultPerPage = 20
	MaxPerPage     = 100
)

// SortSpec whitelists sortable fields. Keys are API names, values are SQL
// expressions; user input never reaches SQL directly.
type SortSpec struct {
	Allowed     map[string]string
	Default     string // API name
	DefaultDesc bool
}

// PageQuery is the parsed ?page&per_page&q&sort&order query.
type PageQuery struct {
	Page    int
	PerPage int
	Search  string
	sortSQL string
	desc    bool
}

type PageMeta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// ParsePageQuery validates pagination, search and sort parameters.
func ParsePageQuery(r *http.Request, spec SortSpec) (PageQuery, error) {
	v := r.URL.Query()
	q := PageQuery{Page: 1, PerPage: DefaultPerPage, Search: strings.TrimSpace(v.Get("q"))}

	if s := v.Get("page"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			return q, BadRequest("Parameter page tidak valid.")
		}
		q.Page = n
	}
	if s := v.Get("per_page"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			return q, BadRequest("Parameter per_page tidak valid.")
		}
		q.PerPage = min(n, MaxPerPage)
	}
	if len(q.Search) > 100 {
		q.Search = q.Search[:100]
	}

	sortKey := v.Get("sort")
	if sortKey == "" {
		sortKey = spec.Default
		q.desc = spec.DefaultDesc
	}
	col, ok := spec.Allowed[sortKey]
	if !ok {
		return q, BadRequest("Parameter sort tidak valid.")
	}
	q.sortSQL = col

	switch strings.ToLower(v.Get("order")) {
	case "":
	case "asc":
		q.desc = false
	case "desc":
		q.desc = true
	default:
		return q, BadRequest("Parameter order harus asc atau desc.")
	}
	return q, nil
}

func (q PageQuery) Offset() int { return (q.Page - 1) * q.PerPage }

// OrderBy returns a safe ORDER BY expression built from the whitelist.
func (q PageQuery) OrderBy() string {
	if q.desc {
		return q.sortSQL + " DESC"
	}
	return q.sortSQL + " ASC"
}

func NewPageMeta(q PageQuery, total int64) PageMeta {
	pages := int((total + int64(q.PerPage) - 1) / int64(q.PerPage))
	return PageMeta{Page: q.Page, PerPage: q.PerPage, Total: total, TotalPages: pages}
}

// LikePattern escapes LIKE wildcards and wraps s for a contains-search.
func LikePattern(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(s) + "%"
}

// OptionalBool parses an optional boolean query parameter.
func OptionalBool(r *http.Request, name string) (*bool, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return nil, nil
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return nil, BadRequest("Parameter " + name + " tidak valid.")
	}
	return &b, nil
}
