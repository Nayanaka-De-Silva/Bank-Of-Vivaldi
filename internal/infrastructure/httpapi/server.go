// Package httpapi is the JSON API surface for Bank of Vivaldi. It calls into
// the same internal/application services as internal/infrastructure/httpui —
// no vault/item/compendium business logic is duplicated here, only request
// parsing, DTO mapping, and error-to-HTTP translation.
//
// v1 has no authentication. Access control is the Docker network boundary:
// this API must never be published on a port reachable from outside the
// trusted host network. See README.md's API section for the follow-up plan
// if external reachability is ever needed.
package httpapi

import (
	"net/http"

	"bank-of-vivaldi/internal/application"
)

type Server struct {
	service *application.Service
}

func NewServer(service *application.Service) *Server {
	return &Server{service: service}
}

// Routes returns the API's handler tree, rooted at "/" (no version prefix —
// the caller mounts this under "/api/v1/" and strips the prefix).
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", s.handleHealth)

	mux.HandleFunc("GET /vaults", handle(s.handleListVaults))
	mux.HandleFunc("POST /vaults", handle(s.handleCreateVault))
	mux.HandleFunc("GET /vaults/{id}", handle(s.handleGetVault))
	mux.HandleFunc("GET /vaults/{id}/items", handle(s.handleBrowseVault))
	mux.HandleFunc("POST /vaults/{id}/items", handle(s.handleAddVaultItem))
	mux.HandleFunc("GET /vaults/{id}/links", handle(s.handleListVaultLinks))
	mux.HandleFunc("POST /vaults/{id}/link", handle(s.handleLinkVault))
	mux.HandleFunc("DELETE /vaults/{id}/link", handle(s.handleUnlinkVault))

	mux.HandleFunc("GET /compendium/items", handle(s.handleBrowseCompendium))

	// No DELETE /vaults/{id}: vault deletion is intentionally UI-only, a
	// deliberate action rather than something an API call can trigger.

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
