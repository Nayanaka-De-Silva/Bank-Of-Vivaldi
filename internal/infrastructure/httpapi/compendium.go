package httpapi

import (
	"net/http"

	"bank-of-vivaldi/internal/application"
)

// parseCompendiumFilters maps query params onto application.CompendiumFilters,
// mirroring internal/infrastructure/httpui's parseCompendiumFilters param
// names 1:1 so both surfaces accept the same filter vocabulary. The actual
// filter behavior lives in application/domain — this is param extraction
// only, not filter logic.
func parseCompendiumFilters(r *http.Request) application.CompendiumFilters {
	q := r.URL.Query()
	return application.CompendiumFilters{
		Query:                  q.Get("q"),
		Category:               q.Get("category"),
		Rarity:                 q.Get("rarity"),
		MinWeightLB:            q.Get("minWeightLb"),
		MaxWeightLB:            q.Get("maxWeightLb"),
		MinValueGP:             q.Get("minValueGp"),
		MaxValueGP:             q.Get("maxValueGp"),
		Magical:                q.Get("magical"),
		Attunement:             q.Get("attunement"),
		SortBy:                 q.Get("sort"),
		ArmorCategory:          q.Get("armorCategory"),
		ArmorDexBehavior:       q.Get("armorDexBehavior"),
		ArmorMinAC:             q.Get("armorMinAc"),
		ArmorMaxAC:             q.Get("armorMaxAc"),
		ArmorStealth:           q.Get("armorStealth"),
		WeaponCategory:         q.Get("weaponCategory"),
		WeaponDamageType:       q.Get("weaponDamageType"),
		WeaponProperty:         q.Get("weaponProperty"),
		ContainerMinCapacityLB: q.Get("containerMinCapacityLb"),
		ContainerMaxCapacityLB: q.Get("containerMaxCapacityLb"),
		ToolCategory:           q.Get("toolCategory"),
		MountType:              q.Get("mountType"),
		MountMinSpeed:          q.Get("mountMinSpeed"),
		MountMaxSpeed:          q.Get("mountMaxSpeed"),
		VehicleType:            q.Get("vehicleType"),
		VehicleMinSpeed:        q.Get("vehicleMinSpeed"),
		VehicleMaxSpeed:        q.Get("vehicleMaxSpeed"),
		TreasureKind:           q.Get("treasureKind"),
	}
}

func (s *Server) handleBrowseCompendium(w http.ResponseWriter, r *http.Request) error {
	filters := parseCompendiumFilters(r)

	browse, err := s.service.BrowseCompendium(r.Context(), filters)
	if err != nil {
		return err
	}

	page, pageSize := pagination(r)
	pageEntries := slicePage(browse.Entries, page, pageSize)

	responses := make([]itemEntryResponse, len(pageEntries))
	for i, entry := range pageEntries {
		responses[i] = newItemEntryResponse(entry)
	}

	writeList(w, r, len(browse.Entries), page, pageSize, responses)
	return nil
}

func (s *Server) handleBrowseVault(w http.ResponseWriter, r *http.Request) error {
	vaultID := r.PathValue("id")
	filters := parseCompendiumFilters(r)

	browse, err := s.service.BrowseVault(r.Context(), vaultID, filters)
	if err != nil {
		return err
	}

	page, pageSize := pagination(r)
	pageEntries := slicePage(browse.Entries, page, pageSize)

	responses := make([]itemEntryResponse, len(pageEntries))
	for i, entry := range pageEntries {
		responses[i] = newItemEntryResponse(entry)
	}

	writeList(w, r, len(browse.Entries), page, pageSize, responses)
	return nil
}
