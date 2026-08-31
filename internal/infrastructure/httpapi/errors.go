package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"bank-of-vivaldi/internal/domain"
)

// FieldError names a single invalid request field, e.g. from DTO validation.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// APIError is the one error type every httpapi handler is expected to return.
// writeError is the one place that turns it into an HTTP response, mirroring a
// single onError hook rather than per-route try/catch.
type APIError struct {
	Status  int          `json:"-"`
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []FieldError `json:"details,omitempty"`
}

func (e *APIError) Error() string { return e.Message }

func badRequest(field, message string) *APIError {
	return &APIError{
		Status:  http.StatusBadRequest,
		Code:    "INVALID_INPUT",
		Message: "request validation failed",
		Details: []FieldError{{Field: field, Message: message}},
	}
}

func validationError(details []FieldError) *APIError {
	return &APIError{
		Status:  http.StatusBadRequest,
		Code:    "INVALID_INPUT",
		Message: "request validation failed",
		Details: details,
	}
}

// classify maps a plain error onto an *APIError by checking domain sentinels,
// so application/postgres code never has to know about HTTP status codes.
func classify(err error) *APIError {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}

	switch {
	case errors.Is(err, domain.ErrNotFound):
		return &APIError{Status: http.StatusNotFound, Code: "NOT_FOUND", Message: err.Error()}
	case errors.Is(err, domain.ErrInvalidInput):
		return &APIError{Status: http.StatusBadRequest, Code: "INVALID_INPUT", Message: err.Error()}
	case errors.Is(err, domain.ErrConflict):
		return &APIError{Status: http.StatusConflict, Code: "CONFLICT", Message: err.Error()}
	default:
		// Unknown errors (DB drivers, I/O, etc.) never leak past this boundary —
		// log the real cause server-side, return a generic message to the caller.
		log.Printf("httpapi: internal error: %v", err)
		return &APIError{Status: http.StatusInternalServerError, Code: "INTERNAL_ERROR", Message: "internal server error"}
	}
}

func writeError(w http.ResponseWriter, err error) {
	apiErr := classify(err)
	writeJSON(w, apiErr.Status, map[string]*APIError{"error": apiErr})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("httpapi: encode response: %v", err)
	}
}

// handle adapts an error-returning handler function to http.HandlerFunc,
// routing any returned error through writeError. Handlers that write their own
// success response and return nil are left untouched.
func handle(fn func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			writeError(w, err)
		}
	}
}
