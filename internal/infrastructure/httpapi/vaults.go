package httpapi

import (
	"encoding/json"
	"net/http"

	"bank-of-vivaldi/internal/application"
)

func (s *Server) handleListVaults(w http.ResponseWriter, r *http.Request) error {
	summaries, _, err := s.service.ListVaults(r.Context())
	if err != nil {
		return err
	}

	page, pageSize := pagination(r)
	pageSummaries := slicePage(summaries, page, pageSize)

	responses := make([]vaultSummaryResponse, len(pageSummaries))
	for i, summary := range pageSummaries {
		responses[i] = newVaultSummaryResponse(summary)
	}

	writeList(w, r, len(summaries), page, pageSize, responses)
	return nil
}

func (s *Server) handleCreateVault(w http.ResponseWriter, r *http.Request) error {
	var req createVaultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return badRequest("body", "must be valid JSON")
	}
	if errs := req.validate(); len(errs) > 0 {
		return validationError(errs)
	}

	vault, err := s.service.CreateVault(r.Context(), application.CreateVaultInput{
		CharacterName:   req.CharacterName,
		StrengthScore:   req.StrengthScore,
		CarryModifierLB: req.CarryModifierLB,
		Notes:           req.Notes,
	})
	if err != nil {
		return err
	}

	writeJSON(w, http.StatusCreated, newVaultResponse(vault))
	return nil
}

func (s *Server) handleGetVault(w http.ResponseWriter, r *http.Request) error {
	vaultID := r.PathValue("id")

	detail, err := s.service.GetVaultDetail(r.Context(), vaultID)
	if err != nil {
		return err
	}

	links, err := s.service.ListVaultLinks(r.Context(), vaultID)
	if err != nil {
		return err
	}

	rootItems := make([]itemNodeResponse, len(detail.RootItems))
	for i, node := range detail.RootItems {
		rootItems[i] = newItemNodeResponse(node)
	}
	linkResponses := make([]vaultLinkResponse, len(links))
	for i, link := range links {
		linkResponses[i] = newVaultLinkResponse(link)
	}

	writeJSON(w, http.StatusOK, vaultDetailResponse{
		Summary:   newVaultSummaryResponse(detail.Summary),
		RootItems: rootItems,
		Links:     linkResponses,
	})
	return nil
}

func (s *Server) handleListVaultLinks(w http.ResponseWriter, r *http.Request) error {
	vaultID := r.PathValue("id")

	links, err := s.service.ListVaultLinks(r.Context(), vaultID)
	if err != nil {
		return err
	}

	responses := make([]vaultLinkResponse, len(links))
	for i, link := range links {
		responses[i] = newVaultLinkResponse(link)
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": responses})
	return nil
}

func (s *Server) handleLinkVault(w http.ResponseWriter, r *http.Request) error {
	vaultID := r.PathValue("id")

	var req linkVaultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return badRequest("body", "must be valid JSON")
	}
	if errs := req.validate(); len(errs) > 0 {
		return validationError(errs)
	}

	link, err := s.service.LinkVault(r.Context(), application.LinkVaultInput{
		VaultID:     vaultID,
		ExternalRef: req.ExternalRef,
		Label:       req.Label,
	})
	if err != nil {
		return err
	}

	writeJSON(w, http.StatusCreated, newVaultLinkResponse(link))
	return nil
}

func (s *Server) handleUnlinkVault(w http.ResponseWriter, r *http.Request) error {
	vaultID := r.PathValue("id")
	externalRef := r.URL.Query().Get("externalRef")
	if externalRef == "" {
		return badRequest("externalRef", "query parameter is required")
	}

	if err := s.service.UnlinkVault(r.Context(), vaultID, externalRef); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
