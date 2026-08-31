package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bank-of-vivaldi/internal/application"
	"bank-of-vivaldi/internal/domain"
)

func newTestServer() (*Server, *fakeStore) {
	store := newFakeStore()
	svc := application.NewService(store)
	return NewServer(svc), store
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthEndpoint(t *testing.T) {
	server, _ := newTestServer()
	rec := doJSON(t, server.Routes(), http.MethodGet, "/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestCreateVaultThenGetIt(t *testing.T) {
	server, _ := newTestServer()
	h := server.Routes()

	rec := doJSON(t, h, http.MethodPost, "/vaults", createVaultRequest{
		CharacterName: "Bruenor", StrengthScore: 16,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}
	var created vaultResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created vault: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected generated id")
	}

	rec = doJSON(t, h, http.MethodGet, "/vaults/"+created.ID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var detail vaultDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode vault detail: %v", err)
	}
	if detail.Summary.Vault.CharacterName != "Bruenor" {
		t.Fatalf("unexpected character name: %+v", detail)
	}
	if detail.Links == nil {
		t.Fatalf("expected links to be an empty list, not nil")
	}
}

func TestCreateVaultRejectsInvalidInput(t *testing.T) {
	server, _ := newTestServer()
	rec := doJSON(t, server.Routes(), http.MethodPost, "/vaults", createVaultRequest{StrengthScore: 10})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rec.Code, rec.Body.String())
	}
	errObj := decodeErrorEnvelope(t, rec)
	if errObj["code"] != "INVALID_INPUT" {
		t.Fatalf("expected INVALID_INPUT, got %v", errObj["code"])
	}
}

func TestGetVaultUnknownIsNotFound(t *testing.T) {
	server, _ := newTestServer()
	rec := doJSON(t, server.Routes(), http.MethodGet, "/vaults/does-not-exist", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestListVaultsEnvelope(t *testing.T) {
	server, store := newTestServer()
	store.vaults["v1"] = domain.Vault{ID: "v1", CharacterName: "Aria", StrengthScore: 10, EncumbranceMode: domain.EncumbranceModeStandard}
	store.vaults["v2"] = domain.Vault{ID: "v2", CharacterName: "Bram", StrengthScore: 12, EncumbranceMode: domain.EncumbranceModeStandard}

	rec := doJSON(t, server.Routes(), http.MethodGet, "/vaults", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var envelope struct {
		Data []vaultSummaryResponse `json:"data"`
		Meta listMeta               `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode list envelope: %v", err)
	}
	if len(envelope.Data) != 2 {
		t.Fatalf("expected 2 vaults, got %d", len(envelope.Data))
	}
	if envelope.Meta.TotalItems != 2 {
		t.Fatalf("expected totalItems 2, got %d", envelope.Meta.TotalItems)
	}
	// summarizeVaults sorts by character name, so this ordering is deterministic.
	if envelope.Data[0].Vault.CharacterName != "Aria" {
		t.Fatalf("expected Aria first, got %q", envelope.Data[0].Vault.CharacterName)
	}
}

func TestVaultRoutesMethodNotAllowed(t *testing.T) {
	server, _ := newTestServer()
	rec := doJSON(t, server.Routes(), http.MethodPatch, "/vaults", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestLinkAndUnlinkVaultViaHTTP(t *testing.T) {
	server, store := newTestServer()
	store.vaults["v1"] = domain.Vault{ID: "v1", CharacterName: "Aria", StrengthScore: 10, EncumbranceMode: domain.EncumbranceModeStandard}
	h := server.Routes()

	rec := doJSON(t, h, http.MethodPost, "/vaults/v1/link", linkVaultRequest{ExternalRef: "npc-manager:42"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/vaults/v1/links", nil)
	var envelope struct {
		Data []vaultLinkResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(envelope.Data) != 1 {
		t.Fatalf("expected 1 link, got %d", len(envelope.Data))
	}

	rec = doJSON(t, h, http.MethodDelete, "/vaults/v1/link?externalRef=npc-manager:42", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (%s)", rec.Code, rec.Body.String())
	}

	// Vault must survive the unlink.
	rec = doJSON(t, h, http.MethodGet, "/vaults/v1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("vault should still exist after unlink, got %d", rec.Code)
	}
}

func TestLinkVaultUnknownVaultIsNotFound(t *testing.T) {
	server, _ := newTestServer()
	rec := doJSON(t, server.Routes(), http.MethodPost, "/vaults/nope/link", linkVaultRequest{ExternalRef: "x"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (%s)", rec.Code, rec.Body.String())
	}
}
