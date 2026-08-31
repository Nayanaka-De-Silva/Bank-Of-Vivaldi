package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"bank-of-vivaldi/internal/application"
	"bank-of-vivaldi/internal/domain"
)

// handleAddVaultItem places an item into a vault, either by transferring
// (copying or moving) an existing item — typically pulled from the compendium
// — or by defining a brand new item inline. See addVaultItemRequest for the
// two accepted request shapes.
func (s *Server) handleAddVaultItem(w http.ResponseWriter, r *http.Request) error {
	vaultID := r.PathValue("id")

	var req addVaultItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return badRequest("body", "must be valid JSON")
	}
	if errs := req.validate(); len(errs) > 0 {
		return validationError(errs)
	}

	location, err := s.resolveVaultLocation(r.Context(), vaultID, req.ContainerID)
	if err != nil {
		return err
	}

	if req.isTransfer() {
		item, err := s.transferItem(r.Context(), req, location)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusCreated, newItemResponse(item))
		return nil
	}

	item, err := s.service.SaveItem(r.Context(), application.SaveItemInput{
		Name:               req.Name,
		Description:        req.Description,
		Category:           req.Category,
		Subcategory:        req.Subcategory,
		Rarity:             domain.ParseRarity(req.Rarity),
		WeightHundredthsLB: req.WeightHundredthsLB,
		BaseValueCP:        req.BaseValueCP,
		Quantity:           req.Quantity,
		IsStackable:        req.IsStackable,
		SourceKind:         domain.SourceKindManual,
		Location:           location,
	})
	if err != nil {
		return err
	}

	writeJSON(w, http.StatusCreated, newItemResponse(item))
	return nil
}

// transferItem copies or moves an existing item (req.SourceItemID) into
// location. Copy is the default — the common compendium-to-vault path — since
// it leaves the source item (e.g. a shared compendium entry) untouched.
func (s *Server) transferItem(ctx context.Context, req addVaultItemRequest, location domain.ItemLocation) (domain.Item, error) {
	if req.Mode == "move" {
		return s.service.MoveItem(ctx, req.SourceItemID, location)
	}
	return s.service.CopyItem(ctx, req.SourceItemID, location)
}

// resolveVaultLocation builds the destination location for an item entering
// vaultID: the vault root, or nested inside one of that vault's containers.
// When a containerID is given, its ownership is checked explicitly — without
// this, application.resolveLocation would silently derive OwnerVaultID from
// the container's actual parent, letting a request for vault A place an item
// in vault B just by naming one of B's container IDs.
func (s *Server) resolveVaultLocation(ctx context.Context, vaultID, containerID string) (domain.ItemLocation, error) {
	if containerID == "" {
		return domain.ItemLocation{
			Kind:         domain.LocationKindVaultRoot,
			OwnerVaultID: vaultID,
		}, nil
	}

	detail, err := s.service.GetItemDetail(ctx, containerID)
	if err != nil {
		return domain.ItemLocation{}, err
	}
	if !detail.Item.IsContainer {
		return domain.ItemLocation{}, badRequest("containerId", "target item is not a container")
	}
	if detail.Item.Location.OwnerVaultID != vaultID {
		return domain.ItemLocation{}, badRequest("containerId", "target container does not belong to this vault")
	}

	return domain.ItemLocation{
		Kind:                  domain.LocationKindContainer,
		ParentContainerItemID: containerID,
	}, nil
}
