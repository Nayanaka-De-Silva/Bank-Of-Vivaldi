package httpapi

import "net/http"

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

type listMeta struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}

// pagination reads page/pageSize query params with sane defaults and caps, so
// a caller can't request an unbounded page size.
func pagination(r *http.Request) (page, pageSize int) {
	page = queryInt(r, "page", 1)
	if page < 1 {
		page = 1
	}
	pageSize = queryInt(r, "pageSize", defaultPageSize)
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func queryInt(r *http.Request, key string, fallback int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	n := 0
	for _, c := range raw {
		if c < '0' || c > '9' {
			return fallback
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return fallback
	}
	return n
}

// writeList slices items into the requested page and writes the standard
// {"data": [...], "meta": {...}} list envelope.
func writeList[T any](w http.ResponseWriter, r *http.Request, total int, page, pageSize int, pageItems []T) {
	totalPages := total / pageSize
	if total%pageSize != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": pageItems,
		"meta": listMeta{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: total,
			TotalPages: totalPages,
		},
	})
}

// slicePage returns the page-th slice (1-indexed) of size pageSize from items.
func slicePage[T any](items []T, page, pageSize int) []T {
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []T{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}
