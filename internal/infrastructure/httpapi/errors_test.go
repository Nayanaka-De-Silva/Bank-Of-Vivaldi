package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bank-of-vivaldi/internal/domain"
)

func decodeErrorEnvelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v (body=%s)", err, rec.Body.String())
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected top-level %q object, got %s", "error", rec.Body.String())
	}
	return errObj
}

func TestWriteErrorKnownAPIError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, &APIError{Status: http.StatusNotFound, Code: "NOT_FOUND", Message: "vault not found: abc"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected JSON content type, got %q", ct)
	}
	errObj := decodeErrorEnvelope(t, rec)
	if errObj["code"] != "NOT_FOUND" {
		t.Fatalf("expected code NOT_FOUND, got %v", errObj["code"])
	}
}

func TestWriteErrorClassifiesDomainSentinels(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"not found", fmt.Errorf("vault %q: %w", "x", domain.ErrNotFound), http.StatusNotFound, "NOT_FOUND"},
		{"invalid input", fmt.Errorf("bad field: %w", domain.ErrInvalidInput), http.StatusBadRequest, "INVALID_INPUT"},
		{"conflict", fmt.Errorf("capacity: %w", domain.ErrConflict), http.StatusConflict, "CONFLICT"},
		{"unknown", errors.New("boom"), http.StatusInternalServerError, "INTERNAL_ERROR"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeError(rec, tc.err)
			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, rec.Code)
			}
			errObj := decodeErrorEnvelope(t, rec)
			if errObj["code"] != tc.wantCode {
				t.Fatalf("expected code %q, got %v", tc.wantCode, errObj["code"])
			}
		})
	}
}

func TestWriteErrorNeverLeaksInternalDetail(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, errors.New("pgx: connection refused to db:5432"))

	errObj := decodeErrorEnvelope(t, rec)
	message, _ := errObj["message"].(string)
	if strings.Contains(message, "pgx") || strings.Contains(message, "5432") {
		t.Fatalf("internal error detail leaked into response: %q", message)
	}
}

func TestHandleWrapsErrorReturningHandlers(t *testing.T) {
	h := handle(func(w http.ResponseWriter, r *http.Request) error {
		return badRequest("name", "is required")
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/whatever", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	errObj := decodeErrorEnvelope(t, rec)
	details, ok := errObj["details"].([]any)
	if !ok || len(details) != 1 {
		t.Fatalf("expected 1 field detail, got %v", errObj["details"])
	}
}

func TestHandlePassesThroughSuccess(t *testing.T) {
	h := handle(func(w http.ResponseWriter, r *http.Request) error {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "yes"})
		return nil
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/whatever", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
