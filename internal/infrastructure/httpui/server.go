package httpui

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"bank-of-vivaldi/internal/application"
	"bank-of-vivaldi/internal/domain"
	webassets "bank-of-vivaldi/web"
)

type Server struct {
	service *application.Service
	tmpl    *template.Template
	static  http.Handler
}

type TemplateData struct {
	Title                     string
	Notice                    string
	Error                     string
	AppSettings               domain.AppSettings
	Dashboard                 application.DashboardData
	Vaults                    []application.VaultSummary
	VaultDetail               application.VaultDetail
	VaultBrowse               application.ItemBrowse
	VaultView                 string
	ItemDetail                application.ItemDetail
	Compendium                application.CompendiumBrowse
	CompendiumFilters         application.CompendiumFilters
	CompendiumView            string
	FilterBar                 FilterBarData
	VaultFilterBar            VaultFilterBarData
	SearchResults             []application.SearchResult
	BulkPreview               domain.BulkPreview
	BulkText                  string
	EncodedRows               string
	AllVaults                 []domain.Vault
	ContainerOptions          []domain.Item
	Categories                []string
	Rarities                  []string
	Item                      domain.Item
	Filters                   application.SearchFilters
	SelectedLocationKind      string
	SelectedVaultID           string
	SelectedParentContainerID string
	BulkDefaultsCategory      string
	BulkDefaultsRarity        string
	BulkDefaultsWeight        string
	BulkDefaultsValue         int
	BulkIsStackable           bool
	BulkIsMagical             bool
	BulkRequiresAttunement    bool
	BulkIsEquipped            bool
	BulkSourceKind            string
}

const defaultItemFormCategory = "equipment"

func normalizeItemFormCategory(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return defaultItemFormCategory
	}
	return normalized
}

func itemMetadataVisible(category, metadataCategory string) bool {
	return normalizeItemFormCategory(category) == strings.ToLower(strings.TrimSpace(metadataCategory))
}

// rarityClass turns a rarity into a CSS modifier class so cards and badges can be
// colour-coded per rarity tier.
func rarityClass(value any) string {
	return "rarity-" + domain.Slugify(fmt.Sprint(value))
}

func containerMetadataVisible(category string, isContainer bool) bool {
	return isContainer || itemMetadataVisible(category, "container")
}

// FilterBarData is the view model for the item_filter_bar partial shared by
// the compendium and vault browsers. A Go template has a single dot, and the
// filter bar needs filters, facets, vocabulary and its own action/reset URLs
// all at once, so it gets a dedicated struct rather than a funcMap dict hack.
type FilterBarData struct {
	Action     string
	ResetURL   string
	View       string
	Views      []string
	Filters    application.CompendiumFilters
	Facets     domain.CompendiumFacets
	Categories []string
	Rarities   []string
	MatchCount int
	TotalCount int
	// PanelKey identifies the filter fields' own collapsible panel (issue #30),
	// distinct per caller (compendium vs. vault) so localStorage state doesn't
	// collide between the two browsers.
	PanelKey string
}

// VaultFilterBarData is the view model for the vault_filter_bar partial used on
// the dashboard and the vaults list page. It mirrors FilterBarData but carries
// VaultListFilters instead of CompendiumFilters and exposes vault-specific
// vocabulary (VaultKinds, EncumbranceStates).
type VaultFilterBarData struct {
	Action   string
	ResetURL string
	Filters  application.VaultListFilters
	PanelKey string
}

// newVaultFilterBar builds the VaultFilterBarData for a vault list or dashboard.
// ResetURL is always the bare action URL: resetting drops every query parameter,
// which is exactly what an unqualified GET to the action does.
func newVaultFilterBar(action string, filters application.VaultListFilters, panelKey string) VaultFilterBarData {
	return VaultFilterBarData{
		Action:   action,
		ResetURL: action,
		Filters:  filters,
		PanelKey: panelKey,
	}
}

// vaultKindLabel returns the display label for a vault kind string.
func vaultKindLabel(kind any) string {
	k, ok := domain.ParseVaultKind(fmt.Sprint(kind))
	if !ok {
		return fmt.Sprint(kind)
	}
	return k.Label()
}

// vaultFilterPanel builds the panel header for a vault filter bar.
func vaultFilterPanel(level, title string, filters application.VaultListFilters) panelHeader {
	header := panelHeader{Level: level, Title: title}
	count := filters.ActiveFilterCount()
	switch {
	case count == 0:
		header.Meta = "No filters applied"
	case count == 1:
		header.Badge = "1 filter active"
	default:
		header.Badge = fmt.Sprintf("%d filters active", count)
	}
	return header
}

// panelHeader is the view model for the panel_summary partial: the single
// clickable row a collapsed section shrinks to (issue #30). Level keeps the
// document outline intact ("h1" for a page title, "h2" for a sub-section);
// Meta is muted supporting text and Badge is an accent chip for state the DM
// must still see while the panel is minimized.
type panelHeader struct {
	Level string
	Title string
	Meta  string
	Badge string
}

// panel builds a plain section header. Exposed to templates as `panel`.
func panel(level, title, meta string) panelHeader {
	return panelHeader{Level: level, Title: title, Meta: meta}
}

// filterPanel builds the header for a filter section, surfacing how many filters
// are narrowing the results so a minimized filter bar never hides that state.
func filterPanel(level, title string, filters application.CompendiumFilters) panelHeader {
	header := panelHeader{Level: level, Title: title}
	count := filters.ActiveFilterCount()
	switch {
	case count == 0:
		header.Meta = "No filters applied"
	case count == 1:
		header.Badge = "1 filter active"
	default:
		header.Badge = fmt.Sprintf("%d filters active", count)
	}
	return header
}

var compendiumViews = []string{compendiumViewTiles, compendiumViewList}

const (
	vaultViewTiles = "tiles"
	vaultViewList  = "list"
	vaultViewTree  = "tree"
)

var vaultViews = []string{vaultViewTiles, vaultViewList, vaultViewTree}

// normalizeVaultView resolves the ?view= parameter for the vault browser,
// defaulting to tiles. The vault adds a third "tree" mode (the original
// nested inventory view) alongside the tiles/list pair the compendium uses.
func normalizeVaultView(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case vaultViewList:
		return vaultViewList
	case vaultViewTree:
		return vaultViewTree
	default:
		return vaultViewTiles
	}
}

// newFilterBar builds the shared filter bar view model. ResetURL is always the
// bare action URL: resetting means dropping every query parameter, which is
// exactly what an unqualified GET to the action does.
func newFilterBar(action, view string, views []string, filters application.CompendiumFilters, browse application.ItemBrowse, panelKey string) FilterBarData {
	return FilterBarData{
		Action:     action,
		ResetURL:   action,
		View:       view,
		Views:      views,
		Filters:    filters,
		Facets:     browse.Facets,
		Categories: domain.Categories(),
		Rarities:   domain.Rarities(),
		MatchCount: browse.MatchCount,
		TotalCount: browse.TotalCount,
		PanelKey:   panelKey,
	}
}

// checkboxField is the view model for the checkbox_field partial, which
// renders every checkbox in the app as inline "Field name: []" (issue #18,
// comment #345). Value is empty for a simple boolean flag and set for a
// multi-value group such as weapon_properties; Legacy marks an option that
// isn't part of the canonical vocabulary (e.g. a free-text weapon property
// kept from an earlier entry).
type checkboxField struct {
	Label, Name, Value, Hint string
	Checked, Legacy          bool
}

// checkbox builds a simple boolean checkbox field.
func checkbox(label, name string, checked bool) checkboxField {
	return checkboxField{Label: label, Name: name, Checked: checked}
}

// weaponPropertyCheckbox adapts a resolved weapon property choice into a
// checkbox field, carrying its value, description-as-hint, and legacy state.
func weaponPropertyCheckbox(choice domain.WeaponPropertyChoice) checkboxField {
	hint := choice.Description
	if hint == "" {
		hint = "Custom value kept from an earlier entry."
	}
	return checkboxField{
		Label:   choice.Label,
		Name:    "weapon_properties",
		Value:   choice.Key,
		Hint:    hint,
		Checked: choice.Selected,
		Legacy:  !choice.Known,
	}
}

// weaponPropertyChip is one rendered pill plus the id its tooltip is wired to
// through aria-describedby.
type weaponPropertyChip struct{ Label, Description, DescriptionID string }

// weaponPropertyChips resolves stored property values for display, scoping
// tooltip ids to `scope` (pass the item ID) so multiple cards on one page don't
// collide. Returns nil for nil details or no properties so callers can guard
// with {{with}}.
func weaponPropertyChips(scope any, details *domain.WeaponDetails) []weaponPropertyChip {
	if details == nil || len(details.Properties) == 0 {
		return nil
	}
	scopeStr := domain.Slugify(fmt.Sprint(scope))
	props := domain.ResolveWeaponProperties(details.Properties)
	chips := make([]weaponPropertyChip, 0, len(props))
	for _, prop := range props {
		chip := weaponPropertyChip{
			Label:       prop.Label,
			Description: prop.Description,
		}
		// DescriptionID is only set for described (canonical) properties;
		// an empty id is the signal the partial uses to omit the tooltip element.
		if prop.Description != "" {
			chip.DescriptionID = "wp-" + scopeStr + "-" + prop.Key
		}
		chips = append(chips, chip)
	}
	return chips
}

func NewServer(service *application.Service) (*Server, error) {
	funcs := template.FuncMap{
		"eq":                       func(a, b any) bool { return fmt.Sprint(a) == fmt.Sprint(b) },
		"weight":                   func(value int) string { return domain.FormatWeightHundredths(value) },
		"gp":                       func(value int) string { return domain.FormatCopperAsGold(value) },
		"breakdownCP":              domain.BreakdownCP,
		"humanize":                 func(value any) string { return domain.HumanizeLabel(fmt.Sprint(value)) },
		"itemFormCategory":         normalizeItemFormCategory,
		"weaponCategories":         domain.WeaponCategories,
		"rarityClass":              rarityClass,
		"itemMetadataVisible":      itemMetadataVisible,
		"containerMetadataVisible": containerMetadataVisible,
		"contains": func(haystack, needle string) bool {
			return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
		},
		// Nil-safe: returns all ten canonical choices with selection set from the stored properties.
		"weaponPropertyChoices": func(d *domain.WeaponDetails) []domain.WeaponPropertyChoice {
			if d == nil {
				return domain.WeaponPropertyChoices(nil)
			}
			return domain.WeaponPropertyChoices(d.Properties)
		},
		// Returns the ten canonical property keys plus any non-canonical values in stored.
		"weaponPropertyOptions": domain.WeaponPropertyOptions,
		// Resolves stored properties for chip display; returns nil when there are no properties.
		"weaponPropertyChips": weaponPropertyChips,
		// Generates the bulk-import format spec from the live vocabulary.
		"bulkFormatSpec": domain.BulkFormatSpec,
		// Build checkbox_field view models (see checkboxField's doc comment).
		"checkbox":               checkbox,
		"weaponPropertyCheckbox": weaponPropertyCheckbox,
		// Build panel_summary view models for the collapsible sections (issue #30).
		"panel":            panel,
		"filterPanel":      filterPanel,
		"vaultFilterPanel": vaultFilterPanel,
		// Vault-kind helpers for select options and badges.
		"vaultKindLabel": vaultKindLabel,
		"vaultKinds":     domain.VaultKinds,
		// Encumbrance state values for vault filter dropdowns.
		"encumbranceStates": func() []string {
			return []string{
				string(domain.EncumbranceStateNormal),
				string(domain.EncumbranceStateEncumbered),
				string(domain.EncumbranceStateHeavilyEncumbered),
				string(domain.EncumbranceStateOverCapacity),
			}
		},
	}

	tmpl, err := template.New("pages").Funcs(funcs).ParseFS(webassets.FS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	staticFS, err := fs.Sub(webassets.FS, "static")
	if err != nil {
		return nil, err
	}

	return &Server{
		service: service,
		tmpl:    tmpl,
		static:  http.FileServerFS(staticFS),
	}, nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", s.static))
	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/settings/encumbrance", s.handleSetEncumbrance)
	mux.HandleFunc("/vaults", s.handleVaults)
	mux.HandleFunc("/vaults/", s.handleVaultRoutes)
	mux.HandleFunc("/compendium", s.handleCompendium)
	mux.HandleFunc("/items/new", s.handleItemNew)
	mux.HandleFunc("/items", s.handleItems)
	mux.HandleFunc("/items/", s.handleItemRoutes)
	mux.HandleFunc("/bulk", s.handleBulk)
	mux.HandleFunc("/bulk/preview", s.handleBulkPreview)
	mux.HandleFunc("/bulk/commit", s.handleBulkCommit)
	mux.HandleFunc("/search", s.handleSearch)
	return mux
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	filters := parseVaultListFilters(r)
	data, err := s.service.Dashboard(r.Context(), filters)
	if err != nil {
		// A malformed filter value is user error, not a server fault: re-render
		// unfiltered with the message so the DM can correct the field, mirroring
		// handleCompendium's error branch.
		filterErr := err.Error()
		data, err = s.service.Dashboard(r.Context(), application.VaultListFilters{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.render(w, "dashboard", http.StatusBadRequest, TemplateData{
			Title:          "Dashboard",
			Error:          filterErr,
			AppSettings:    data.Settings,
			Dashboard:      data,
			VaultFilterBar: newVaultFilterBar("/", filters, "dashboard-vault-filters"),
		})
		return
	}

	s.render(w, "dashboard", http.StatusOK, TemplateData{
		Title:          "Dashboard",
		Notice:         r.URL.Query().Get("notice"),
		AppSettings:    data.Settings,
		Dashboard:      data,
		VaultFilterBar: newVaultFilterBar("/", filters, "dashboard-vault-filters"),
	})
}

func (s *Server) handleSetEncumbrance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mode := domain.ParseEncumbranceMode(r.FormValue("mode"))
	if err := s.service.SetDefaultEncumbranceMode(r.Context(), mode); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/?notice=Encumbrance+mode+updated", http.StatusSeeOther)
}

func (s *Server) handleVaults(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		filters := parseVaultListFilters(r)
		vaults, settings, err := s.service.ListVaults(r.Context(), filters)
		if err != nil {
			// A malformed filter value is user error, not a server fault:
			// re-render unfiltered with the message, mirroring handleCompendium.
			filterErr := err.Error()
			vaults, settings, err = s.service.ListVaults(r.Context(), application.VaultListFilters{})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			s.render(w, "vaults", http.StatusBadRequest, TemplateData{
				Title:          "Vaults",
				Error:          filterErr,
				AppSettings:    settings,
				Vaults:         vaults,
				VaultFilterBar: newVaultFilterBar("/vaults", filters, "vault-list-filters"),
			})
			return
		}
		s.render(w, "vaults", http.StatusOK, TemplateData{
			Title:          "Vaults",
			Notice:         r.URL.Query().Get("notice"),
			AppSettings:    settings,
			Vaults:         vaults,
			VaultFilterBar: newVaultFilterBar("/vaults", filters, "vault-list-filters"),
		})
	case http.MethodPost:
		strength, err := parseIntField(r.FormValue("strength_score"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		carryModifier, err := parseIntField(r.FormValue("carry_modifier_lb"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		kind, _ := domain.ParseVaultKind(r.FormValue("kind"))
		if _, err := s.service.CreateVault(r.Context(), application.CreateVaultInput{
			CharacterName:   r.FormValue("character_name"),
			StrengthScore:   strength,
			CarryModifierLB: carryModifier,
			Notes:           r.FormValue("notes"),
			Kind:            kind,
		}); err != nil {
			vaults, settings, listErr := s.service.ListVaults(r.Context(), application.VaultListFilters{})
			if listErr != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			s.render(w, "vaults", http.StatusBadRequest, TemplateData{
				Title:          "Vaults",
				Error:          err.Error(),
				AppSettings:    settings,
				Vaults:         vaults,
				VaultFilterBar: newVaultFilterBar("/vaults", application.VaultListFilters{}, "vault-list-filters"),
			})
			return
		}
		http.Redirect(w, r, "/vaults?notice=Vault+created", http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// vaultRouteAction splits a /vaults/{id}[/{action}] path into its parts,
// tolerating a trailing slash. Returns an empty id for a malformed path (bare
// /vaults/ has no id to dispatch on).
func vaultRouteAction(path string) (id, action string) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/vaults/"), "/")
	if trimmed == "" {
		return "", ""
	}
	if slash := strings.Index(trimmed, "/"); slash >= 0 {
		return trimmed[:slash], trimmed[slash+1:]
	}
	return trimmed, ""
}

func (s *Server) handleVaultRoutes(w http.ResponseWriter, r *http.Request) {
	id, action := vaultRouteAction(r.URL.Path)
	if id == "" {
		http.NotFound(w, r)
		return
	}

	switch action {
	case "":
		s.handleVaultDetail(w, r, id)
	case "edit":
		s.handleVaultEdit(w, r, id)
	case "purse":
		s.handleVaultPurse(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleVaultDetail(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method == http.MethodPost {
		strength, err := parseIntField(r.FormValue("strength_score"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		carryModifier, err := parseIntField(r.FormValue("carry_modifier_lb"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		editKind, _ := domain.ParseVaultKind(r.FormValue("kind"))
		_, err = s.service.UpdateVault(r.Context(), application.UpdateVaultInput{
			ID:              id,
			CharacterName:   r.FormValue("character_name"),
			StrengthScore:   strength,
			CarryModifierLB: carryModifier,
			EncumbranceMode: domain.ParseEncumbranceMode(r.FormValue("encumbrance_mode")),
			Kind:            editKind,
			Notes:           r.FormValue("notes"),
			Archived:        r.FormValue("archived") == "on",
		})
		if err != nil {
			data, detailErr := s.service.GetVaultDetail(r.Context(), id)
			if detailErr != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			// Edits live on /vaults/{id}/edit now, so a rejected save must
			// re-render that page -- not the read-only detail page, which no
			// longer has a form on it at all.
			s.render(w, "vault_edit", http.StatusBadRequest, TemplateData{
				Title:       "Edit Vault",
				Error:       err.Error(),
				AppSettings: data.Settings,
				VaultDetail: data,
			})
			return
		}

		http.Redirect(w, r, "/vaults/"+id+"?notice=Vault+updated", http.StatusSeeOther)
		return
	}

	data, err := s.service.GetVaultDetail(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	filters := parseCompendiumFilters(r)
	view := normalizeVaultView(r.URL.Query().Get("view"))
	action := "/vaults/" + id

	browse, err := s.service.BrowseVault(r.Context(), id, filters)
	if err != nil {
		// A malformed filter value is user error, not a server fault: re-render
		// the browser with the message so the DM can correct the field, mirroring
		// handleCompendium's error branch.
		s.render(w, "vault_detail", http.StatusBadRequest, TemplateData{
			Title:       data.Summary.Vault.CharacterName,
			Error:       err.Error(),
			AppSettings: data.Settings,
			VaultDetail: data,
			VaultView:   view,
			FilterBar:   newFilterBar(action, view, vaultViews, filters, application.ItemBrowse{}, "vault-filters"),
		})
		return
	}

	s.render(w, "vault_detail", http.StatusOK, TemplateData{
		Title:       data.Summary.Vault.CharacterName,
		Notice:      r.URL.Query().Get("notice"),
		AppSettings: data.Settings,
		VaultDetail: data,
		VaultBrowse: browse,
		VaultView:   view,
		FilterBar:   newFilterBar(action, view, vaultViews, filters, browse, "vault-filters"),
	})
}

// handleVaultEdit renders the vault settings + purse forms that used to live
// directly on the detail page (issue #18): edits now sit behind this
// dedicated page, reached via an "Edit vault" button. The forms themselves
// still post to the unchanged /vaults/{id} and /vaults/{id}/purse routes.
func (s *Server) handleVaultEdit(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := s.service.GetVaultDetail(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	s.render(w, "vault_edit", http.StatusOK, TemplateData{
		Title:       "Edit " + data.Summary.Vault.CharacterName,
		AppSettings: data.Settings,
		VaultDetail: data,
	})
}

func (s *Server) handleVaultPurse(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	purse, err := parsePurse(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.service.AdjustPurse(r.Context(), id, purse); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/vaults/"+id+"?notice=Purse+updated", http.StatusSeeOther)
}

func (s *Server) handleCompendium(w http.ResponseWriter, r *http.Request) {
	filters := parseCompendiumFilters(r)
	view := normalizeCompendiumView(r.URL.Query().Get("view"))

	browse, err := s.service.BrowseCompendium(r.Context(), filters)
	if err != nil {
		// A malformed filter value is user error, not a server fault: re-render the
		// browser with the message so the DM can correct the field.
		settings, settingsErr := s.service.Settings(r.Context())
		if settingsErr != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.render(w, "compendium", http.StatusBadRequest, TemplateData{
			Title:             "Compendium",
			Error:             err.Error(),
			AppSettings:       settings,
			Categories:        domain.Categories(),
			Rarities:          domain.Rarities(),
			CompendiumFilters: filters,
			CompendiumView:    view,
			FilterBar:         newFilterBar("/compendium", view, compendiumViews, filters, application.ItemBrowse{}, "compendium-filters"),
		})
		return
	}

	s.render(w, "compendium", http.StatusOK, TemplateData{
		Title:             "Compendium",
		Notice:            r.URL.Query().Get("notice"),
		AppSettings:       browse.Settings,
		Categories:        domain.Categories(),
		Rarities:          domain.Rarities(),
		Compendium:        browse,
		CompendiumFilters: filters,
		CompendiumView:    view,
		FilterBar:         newFilterBar("/compendium", view, compendiumViews, filters, browse, "compendium-filters"),
	})
}

const (
	compendiumViewTiles = "tiles"
	compendiumViewList  = "list"
)

// normalizeCompendiumView resolves the ?view= parameter, defaulting to tiles.
func normalizeCompendiumView(value string) string {
	if strings.ToLower(strings.TrimSpace(value)) == compendiumViewList {
		return compendiumViewList
	}
	return compendiumViewTiles
}

// parseVaultListFilters maps the vault list/dashboard query string onto the raw
// filter values the application layer parses and the template repopulates.
func parseVaultListFilters(r *http.Request) application.VaultListFilters {
	query := r.URL.Query()
	return application.VaultListFilters{
		Query:         query.Get("q"),
		Kind:          query.Get("kind"),
		MinCarryLB:    query.Get("min_carry_lb"),
		MaxCarryLB:    query.Get("max_carry_lb"),
		MinCapacityLB: query.Get("min_capacity_lb"),
		MaxCapacityLB: query.Get("max_capacity_lb"),
		MinItems:      query.Get("min_items"),
		MaxItems:      query.Get("max_items"),
		State:         query.Get("state"),
		SortBy:        query.Get("sort"),
	}
}

// parseCompendiumFilters maps the compendium browse query string onto the raw
// filter values the application layer parses and the template repopulates.
func parseCompendiumFilters(r *http.Request) application.CompendiumFilters {
	query := r.URL.Query()
	return application.CompendiumFilters{
		Query:                  query.Get("q"),
		Category:               query.Get("category"),
		Rarity:                 query.Get("rarity"),
		MinWeightLB:            query.Get("min_weight_lb"),
		MaxWeightLB:            query.Get("max_weight_lb"),
		MinValueGP:             query.Get("min_value_gp"),
		MaxValueGP:             query.Get("max_value_gp"),
		Magical:                query.Get("magical"),
		Attunement:             query.Get("attunement"),
		SortBy:                 query.Get("sort"),
		ArmorCategory:          query.Get("armor_category"),
		ArmorDexBehavior:       query.Get("armor_dex_behavior"),
		ArmorMinAC:             query.Get("armor_min_ac"),
		ArmorMaxAC:             query.Get("armor_max_ac"),
		ArmorStealth:           query.Get("armor_stealth"),
		WeaponCategory:         query.Get("weapon_category"),
		WeaponDamageType:       query.Get("weapon_damage_type"),
		WeaponProperty:         query.Get("weapon_property"),
		ContainerMinCapacityLB: query.Get("container_min_capacity_lb"),
		ContainerMaxCapacityLB: query.Get("container_max_capacity_lb"),
		ToolCategory:           query.Get("tool_category"),
		MountType:              query.Get("mount_type"),
		MountMinSpeed:          query.Get("mount_min_speed"),
		MountMaxSpeed:          query.Get("mount_max_speed"),
		VehicleType:            query.Get("vehicle_type"),
		VehicleMinSpeed:        query.Get("vehicle_min_speed"),
		VehicleMaxSpeed:        query.Get("vehicle_max_speed"),
		TreasureKind:           query.Get("treasure_kind"),
	}
}

func (s *Server) handleItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	input, err := parseItemInput(r, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	item, err := s.service.SaveItem(r.Context(), input)
	if err != nil {
		s.renderItemFormError(w, r, http.StatusBadRequest, err.Error(), domain.Item{})
		return
	}

	http.Redirect(w, r, "/items/"+item.ID+"?notice=Item+created", http.StatusSeeOther)
}

func (s *Server) handleItemNew(w http.ResponseWriter, r *http.Request) {
	settings, err := s.service.Settings(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	vaults, containers, err := s.loadFormOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render(w, "item_form", http.StatusOK, TemplateData{
		Title:                "Create Item",
		AppSettings:          settings,
		AllVaults:            vaults,
		ContainerOptions:     containers,
		Categories:           domain.Categories(),
		Rarities:             domain.Rarities(),
		SelectedLocationKind: string(domain.LocationKindCompendiumRoot),
	})
}

func (s *Server) handleItemRoutes(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/delete"):
		s.handleItemDelete(w, r)
	case strings.HasSuffix(r.URL.Path, "/move"):
		s.handleItemMove(w, r)
	case strings.HasSuffix(r.URL.Path, "/copy"):
		s.handleItemCopy(w, r)
	case strings.HasSuffix(r.URL.Path, "/split"):
		s.handleItemSplit(w, r)
	case strings.HasSuffix(r.URL.Path, "/merge"):
		s.handleItemMerge(w, r)
	case strings.HasSuffix(r.URL.Path, "/edit"):
		s.handleItemEdit(w, r)
	default:
		s.handleItemDetail(w, r)
	}
}

func (s *Server) handleItemDetail(w http.ResponseWriter, r *http.Request) {
	id := pathSegment(r.URL.Path, "/items/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	if r.Method == http.MethodPost {
		input, err := parseItemInput(r, id)
		if err == nil {
			_, err = s.service.SaveItem(r.Context(), input)
		}
		if err != nil {
			// Edits live on /items/{id}/edit now, so a rejected save must
			// re-render that page -- not the read-only detail page, which no
			// longer has a form on it at all.
			s.renderItemEditError(w, r, id, err.Error())
			return
		}
		http.Redirect(w, r, "/items/"+id+"?notice=Item+updated", http.StatusSeeOther)
		return
	}

	detail, err := s.service.GetItemDetail(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	vaults, containers, err := s.loadFormOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render(w, "item_detail", http.StatusOK, TemplateData{
		Title:                     detail.Item.Name,
		Notice:                    r.URL.Query().Get("notice"),
		AppSettings:               detail.Settings,
		ItemDetail:                detail,
		Item:                      detail.Item,
		AllVaults:                 vaults,
		ContainerOptions:          containers,
		SelectedLocationKind:      string(detail.Item.Location.Kind),
		SelectedVaultID:           detail.Item.Location.OwnerVaultID,
		SelectedParentContainerID: detail.Item.Location.ParentContainerItemID,
	})
}

// handleItemEdit renders the item edit form that used to live directly on the
// detail page (issue #27, the same split #18 did for vaults): edits now sit
// behind this dedicated page, reached via an "Edit item" button. The form
// still posts to the unchanged /items/{id} route.
func (s *Server) handleItemEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := pathSegment(strings.TrimSuffix(r.URL.Path, "/edit"), "/items/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	detail, err := s.service.GetItemDetail(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	vaults, containers, err := s.loadFormOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render(w, "item_edit", http.StatusOK, itemEditTemplateData(detail, vaults, containers, ""))
}

// renderItemEditError re-renders the item edit page with a validation error
// after a rejected save, mirroring handleVaultDetail's equivalent branch.
func (s *Server) renderItemEditError(w http.ResponseWriter, r *http.Request, id, errText string) {
	detail, err := s.service.GetItemDetail(r.Context(), id)
	if err != nil {
		http.Error(w, errText, http.StatusBadRequest)
		return
	}
	vaults, containers, err := s.loadFormOptions(r)
	if err != nil {
		http.Error(w, errText, http.StatusBadRequest)
		return
	}
	s.render(w, "item_edit", http.StatusBadRequest, itemEditTemplateData(detail, vaults, containers, errText))
}

func itemEditTemplateData(detail application.ItemDetail, vaults []domain.Vault, containers []domain.Item, errText string) TemplateData {
	return TemplateData{
		Title:                     "Edit " + detail.Item.Name,
		Error:                     errText,
		AppSettings:               detail.Settings,
		ItemDetail:                detail,
		Item:                      detail.Item,
		AllVaults:                 vaults,
		ContainerOptions:          containers,
		Categories:                domain.Categories(),
		Rarities:                  domain.Rarities(),
		SelectedLocationKind:      string(detail.Item.Location.Kind),
		SelectedVaultID:           detail.Item.Location.OwnerVaultID,
		SelectedParentContainerID: detail.Item.Location.ParentContainerItemID,
	}
}

func (s *Server) handleItemDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := pathSegment(strings.TrimSuffix(r.URL.Path, "/delete"), "/items/")
	if err := s.service.DeleteItem(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/search?notice=Item+deleted", http.StatusSeeOther)
}

func (s *Server) handleItemMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := pathSegment(strings.TrimSuffix(r.URL.Path, "/move"), "/items/")
	location := domain.ItemLocation{
		Kind:                  domain.LocationKind(r.FormValue("location_kind")),
		OwnerVaultID:          strings.TrimSpace(r.FormValue("vault_id")),
		ParentContainerItemID: strings.TrimSpace(r.FormValue("parent_container_item_id")),
	}
	if _, err := s.service.MoveItem(r.Context(), id, location); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/items/"+id+"?notice=Item+moved", http.StatusSeeOther)
}

func (s *Server) handleItemCopy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := pathSegment(strings.TrimSuffix(r.URL.Path, "/copy"), "/items/")
	location := domain.ItemLocation{
		Kind:                  domain.LocationKind(r.FormValue("location_kind")),
		OwnerVaultID:          strings.TrimSpace(r.FormValue("vault_id")),
		ParentContainerItemID: strings.TrimSpace(r.FormValue("parent_container_item_id")),
	}
	copied, err := s.service.CopyItem(r.Context(), id, location)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/items/"+copied.ID+"?notice=Item+copied", http.StatusSeeOther)
}

func (s *Server) handleItemSplit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := pathSegment(strings.TrimSuffix(r.URL.Path, "/split"), "/items/")
	quantity, err := parseIntField(r.FormValue("quantity"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.service.SplitStack(r.Context(), id, quantity); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/items/"+id+"?notice=Stack+split", http.StatusSeeOther)
}

func (s *Server) handleItemMerge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := pathSegment(strings.TrimSuffix(r.URL.Path, "/merge"), "/items/")
	targetID := r.FormValue("target_item_id")
	if err := s.service.MergeStacks(r.Context(), id, targetID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/items/"+targetID+"?notice=Stacks+merged", http.StatusSeeOther)
}

func (s *Server) handleBulk(w http.ResponseWriter, r *http.Request) {
	settings, err := s.service.Settings(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	vaults, containers, err := s.loadFormOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "bulk", http.StatusOK, TemplateData{
		Title:                "Bulk Item Entry",
		Notice:               r.URL.Query().Get("notice"),
		AppSettings:          settings,
		AllVaults:            vaults,
		ContainerOptions:     containers,
		Categories:           domain.Categories(),
		Rarities:             domain.Rarities(),
		SelectedLocationKind: string(domain.LocationKindCompendiumRoot),
		BulkDefaultsRarity:   string(domain.RarityMundane),
	})
}

func (s *Server) handleBulkPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	settings, err := s.service.Settings(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	vaults, containers, err := s.loadFormOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	weight, err := domain.ParseWeightHundredths(r.FormValue("default_weight_lb"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	value, err := parseCoinValue(r, "default_")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	preview, err := s.service.PreviewBulk(r.Context(), r.FormValue("bulk_text"), domain.BulkDefaults{
		Category:           strings.TrimSpace(r.FormValue("default_category")),
		Rarity:             domain.ParseRarity(r.FormValue("default_rarity")),
		WeightHundredthsLB: weight,
		BaseValueCP:        value,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	encoded, err := s.service.EncodeBulkRows(preview.Rows)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render(w, "bulk", http.StatusOK, TemplateData{
		Title:                     "Bulk Item Entry",
		AppSettings:               settings,
		AllVaults:                 vaults,
		ContainerOptions:          containers,
		Categories:                domain.Categories(),
		Rarities:                  domain.Rarities(),
		BulkPreview:               preview,
		EncodedRows:               encoded,
		BulkText:                  r.FormValue("bulk_text"),
		SelectedLocationKind:      r.FormValue("location_kind"),
		SelectedVaultID:           r.FormValue("vault_id"),
		SelectedParentContainerID: r.FormValue("parent_container_item_id"),
		BulkDefaultsCategory:      r.FormValue("default_category"),
		BulkDefaultsRarity:        r.FormValue("default_rarity"),
		BulkDefaultsWeight:        r.FormValue("default_weight_lb"),
		BulkDefaultsValue:         value,
		BulkIsStackable:           r.FormValue("is_stackable") == "on",
		BulkIsMagical:             r.FormValue("is_magical") == "on",
		BulkRequiresAttunement:    r.FormValue("requires_attunement") == "on",
		BulkIsEquipped:            r.FormValue("is_equipped") == "on",
		BulkSourceKind:            r.FormValue("source_kind"),
	})
}

func (s *Server) handleBulkCommit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := s.service.CommitBulk(r.Context(), application.BulkCommitInput{
		EncodedRows:           r.FormValue("encoded_rows"),
		LocationKind:          domain.LocationKind(r.FormValue("location_kind")),
		VaultID:               strings.TrimSpace(r.FormValue("vault_id")),
		ParentContainerItemID: strings.TrimSpace(r.FormValue("parent_container_item_id")),
		IsStackable:           r.FormValue("is_stackable") == "on",
		IsMagical:             r.FormValue("is_magical") == "on",
		RequiresAttunement:    r.FormValue("requires_attunement") == "on",
		IsEquipped:            r.FormValue("is_equipped") == "on",
		SourceKind:            domain.ParseSourceKind(r.FormValue("source_kind")),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/bulk?notice=Bulk+items+created", http.StatusSeeOther)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	settings, err := s.service.Settings(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filters := application.SearchFilters{
		Query:     r.URL.Query().Get("q"),
		Category:  r.URL.Query().Get("category"),
		Rarity:    r.URL.Query().Get("rarity"),
		VaultID:   r.URL.Query().Get("vault_id"),
		Container: r.URL.Query().Get("container"),
		SortBy:    r.URL.Query().Get("sort"),
	}

	results, _, err := s.service.SearchInventory(r.Context(), filters)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	vaults, _, err := s.loadFormOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render(w, "search", http.StatusOK, TemplateData{
		Title:         "Search Inventory",
		Notice:        r.URL.Query().Get("notice"),
		AppSettings:   settings,
		SearchResults: results,
		Filters:       filters,
		AllVaults:     vaults,
		Categories:    domain.Categories(),
		Rarities:      domain.Rarities(),
	})
}

func (s *Server) render(w http.ResponseWriter, name string, status int, data TemplateData) {
	var buffer bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buffer, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write(buffer.Bytes())
}

func (s *Server) loadFormOptions(r *http.Request) ([]domain.Vault, []domain.Item, error) {
	vaults, err := s.service.AllVaults(r.Context())
	if err != nil {
		return nil, nil, err
	}
	containers, err := s.service.ListContainers(r.Context())
	if err != nil {
		return nil, nil, err
	}
	return vaults, containers, nil
}

func (s *Server) renderItemFormError(w http.ResponseWriter, r *http.Request, status int, errText string, item domain.Item) {
	settings, err := s.service.Settings(r.Context())
	if err != nil {
		http.Error(w, errText, status)
		return
	}
	vaults, containers, formErr := s.loadFormOptions(r)
	if formErr != nil {
		http.Error(w, errText, status)
		return
	}
	s.render(w, "item_form", status, TemplateData{
		Title:                     "Create Item",
		Error:                     errText,
		AppSettings:               settings,
		AllVaults:                 vaults,
		ContainerOptions:          containers,
		Categories:                domain.Categories(),
		Rarities:                  domain.Rarities(),
		Item:                      item,
		SelectedLocationKind:      string(item.Location.Kind),
		SelectedVaultID:           item.Location.OwnerVaultID,
		SelectedParentContainerID: item.Location.ParentContainerItemID,
	})
}

func parseItemInput(r *http.Request, existingID string) (application.SaveItemInput, error) {
	category := r.FormValue("category")
	isContainer := r.FormValue("is_container") == "on"

	weight, err := domain.ParseWeightHundredths(r.FormValue("weight_lb"))
	if err != nil {
		return application.SaveItemInput{}, err
	}
	value, err := parseCoinValue(r, "value_")
	if err != nil {
		return application.SaveItemInput{}, err
	}
	quantity, err := parseIntField(r.FormValue("quantity"))
	if err != nil {
		return application.SaveItemInput{}, err
	}

	location := domain.ItemLocation{
		Kind:                  domain.LocationKind(strings.TrimSpace(r.FormValue("location_kind"))),
		OwnerVaultID:          strings.TrimSpace(r.FormValue("vault_id")),
		ParentContainerItemID: strings.TrimSpace(r.FormValue("parent_container_item_id")),
	}

	details, err := parseItemDetails(r, category, isContainer)
	if err != nil {
		return application.SaveItemInput{}, err
	}

	return application.SaveItemInput{
		ID:                 existingID,
		Name:               r.FormValue("name"),
		Description:        r.FormValue("description"),
		Category:           category,
		Subcategory:        r.FormValue("subcategory"),
		Rarity:             domain.ParseRarity(r.FormValue("rarity")),
		WeightHundredthsLB: weight,
		BaseValueCP:        value,
		Quantity:           quantity,
		IsContainer:        isContainer,
		IsStackable:        r.FormValue("is_stackable") == "on",
		IsEquipped:         r.FormValue("is_equipped") == "on",
		IsMagical:          r.FormValue("is_magical") == "on",
		RequiresAttunement: r.FormValue("requires_attunement") == "on",
		SourceKind:         domain.ParseSourceKind(r.FormValue("source_kind")),
		Details:            details,
		Location:           location,
	}, nil
}

func parseItemDetails(r *http.Request, category string, isContainer bool) (domain.ItemDetails, error) {
	// Ensure r.Form is populated before any r.Form[] slice reads (r.FormValue already
	// calls this internally, but being explicit prevents subtle ordering bugs).
	if err := r.ParseForm(); err != nil {
		return domain.ItemDetails{}, err
	}

	var details domain.ItemDetails
	activeCategory := strings.ToLower(strings.TrimSpace(category))

	if isContainer || activeCategory == "container" {
		containerWeight, err := domain.ParseWeightHundredths(r.FormValue("container_max_weight_lb"))
		if err != nil {
			return domain.ItemDetails{}, err
		}
		if containerWeight > 0 {
			details.Container = &domain.ContainerDetails{MaxWeightHundredthsLB: containerWeight}
		}
	}

	if activeCategory == "armor" {
		armorCategory := strings.TrimSpace(r.FormValue("armor_category"))
		baseAC, err := parseIntField(r.FormValue("armor_base_ac"))
		if err != nil {
			return domain.ItemDetails{}, err
		}
		strReq, err := parseIntField(r.FormValue("armor_strength_requirement"))
		if err != nil {
			return domain.ItemDetails{}, err
		}
		if armorCategory != "" {
			details.Armor = &domain.ArmorDetails{
				ArmorCategory:       armorCategory,
				BaseAC:              baseAC,
				DexModifierBehavior: r.FormValue("armor_dex_behavior"),
				StrengthRequirement: strReq,
				StealthDisadvantage: r.FormValue("armor_stealth_disadvantage") == "on",
			}
		}
	}

	if activeCategory == "weapon" {
		weaponClass := strings.TrimSpace(r.FormValue("weapon_class"))
		// Enforce the four-option constraint server-side; the dropdown alone can be bypassed.
		if weaponClass != "" && !domain.IsValidWeaponCategory(weaponClass) {
			return domain.ItemDetails{}, fmt.Errorf("invalid weapon category %q", weaponClass)
		}
		normalRange, err := parseIntField(r.FormValue("weapon_normal_range"))
		if err != nil {
			return domain.ItemDetails{}, err
		}
		longRange, err := parseIntField(r.FormValue("weapon_long_range"))
		if err != nil {
			return domain.ItemDetails{}, err
		}
		// Checkboxes submit repeated values; NormalizeWeaponProperties canonicalizes and dedupes.
		// Unknown/legacy property values are kept as-is (unlike weapon_class, which we reject
		// when invalid) because legacy free-text properties re-submit via pre-checked form entries
		// and must survive a save without being silently dropped.
		properties := domain.NormalizeWeaponProperties(r.Form["weapon_properties"])
		// Set Weapon when the class, any property, or a versatile die is present, so
		// none of them are lost when the user leaves the category dropdown empty.
		versatileDamageDice := strings.TrimSpace(r.FormValue("weapon_versatile_damage_dice"))
		if weaponClass != "" || len(properties) > 0 || versatileDamageDice != "" {
			details.Weapon = &domain.WeaponDetails{
				WeaponClass:         weaponClass,
				DamageDice:          r.FormValue("weapon_damage_dice"),
				DamageType:          r.FormValue("weapon_damage_type"),
				Properties:          properties,
				NormalRange:         normalRange,
				LongRange:           longRange,
				VersatileDamageDice: versatileDamageDice,
			}
		}
	}

	if activeCategory == "tool" {
		if toolCategory := strings.TrimSpace(r.FormValue("tool_category")); toolCategory != "" {
			details.Tool = &domain.ToolDetails{
				ToolCategory:     toolCategory,
				ProficiencyNotes: r.FormValue("tool_proficiency_notes"),
			}
		}
	}

	if activeCategory == "mount" {
		mountType := strings.TrimSpace(r.FormValue("mount_type"))
		speed, err := parseIntField(r.FormValue("mount_speed"))
		if err != nil {
			return domain.ItemDetails{}, err
		}
		carryWeight, err := domain.ParseWeightHundredths(r.FormValue("mount_carrying_capacity_lb"))
		if err != nil {
			return domain.ItemDetails{}, err
		}
		if mountType != "" {
			details.Mount = &domain.MountDetails{
				MountType:                    mountType,
				MovementSpeed:                speed,
				CarryingCapacityHundredthsLB: carryWeight,
			}
		}
	}

	if activeCategory == "vehicle" {
		vehicleType := strings.TrimSpace(r.FormValue("vehicle_type"))
		speed, err := parseIntField(r.FormValue("vehicle_speed"))
		if err != nil {
			return domain.ItemDetails{}, err
		}
		carryWeight, err := domain.ParseWeightHundredths(r.FormValue("vehicle_carrying_capacity_lb"))
		if err != nil {
			return domain.ItemDetails{}, err
		}
		if vehicleType != "" {
			details.Vehicle = &domain.VehicleDetails{
				VehicleType:                  vehicleType,
				MovementSpeed:                speed,
				CarryingCapacityHundredthsLB: carryWeight,
			}
		}
	}

	if activeCategory == "treasure" {
		if treasureKind := strings.TrimSpace(r.FormValue("treasure_kind")); treasureKind != "" {
			details.Treasure = &domain.TreasureDetails{TreasureKind: treasureKind}
		}
	}

	return details, nil
}

func parsePurse(r *http.Request) (domain.Purse, error) {
	cp, err := parseIntField(r.FormValue("cp"))
	if err != nil {
		return domain.Purse{}, err
	}
	sp, err := parseIntField(r.FormValue("sp"))
	if err != nil {
		return domain.Purse{}, err
	}
	ep, err := parseIntField(r.FormValue("ep"))
	if err != nil {
		return domain.Purse{}, err
	}
	gp, err := parseIntField(r.FormValue("gp"))
	if err != nil {
		return domain.Purse{}, err
	}
	pp, err := parseIntField(r.FormValue("pp"))
	if err != nil {
		return domain.Purse{}, err
	}
	return domain.Purse{CP: cp, SP: sp, EP: ep, GP: gp, PP: pp}, nil
}

// parseCoinValue reads multi-denomination coin inputs (prefix+pp/gp/ep/sp/cp)
// and returns the total in copper pieces.
func parseCoinValue(r *http.Request, prefix string) (int, error) {
	pp, err := parseIntField(r.FormValue(prefix + "pp"))
	if err != nil {
		return 0, err
	}
	gp, err := parseIntField(r.FormValue(prefix + "gp"))
	if err != nil {
		return 0, err
	}
	ep, err := parseIntField(r.FormValue(prefix + "ep"))
	if err != nil {
		return 0, err
	}
	sp, err := parseIntField(r.FormValue(prefix + "sp"))
	if err != nil {
		return 0, err
	}
	cp, err := parseIntField(r.FormValue(prefix + "cp"))
	if err != nil {
		return 0, err
	}
	return (pp * 1000) + (gp * 100) + (ep * 50) + (sp * 10) + cp, nil
}

func parseIntField(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q", value)
	}
	return parsed, nil
}

func pathSegment(path, prefix string) string {
	value := strings.TrimPrefix(path, prefix)
	value = strings.Trim(value, "/")
	if value == "" {
		return ""
	}
	if slash := strings.Index(value, "/"); slash >= 0 {
		return value[:slash]
	}
	return value
}
