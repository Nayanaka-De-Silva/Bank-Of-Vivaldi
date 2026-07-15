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
	ItemDetail                application.ItemDetail
	CompendiumItems           []application.ItemNode
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

func containerMetadataVisible(category string, isContainer bool) bool {
	return isContainer || itemMetadataVisible(category, "container")
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
		"itemMetadataVisible":      itemMetadataVisible,
		"containerMetadataVisible": containerMetadataVisible,
		"contains": func(haystack, needle string) bool {
			return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
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

	data, err := s.service.Dashboard(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render(w, "dashboard", http.StatusOK, TemplateData{
		Title:       "Dashboard",
		Notice:      r.URL.Query().Get("notice"),
		AppSettings: data.Settings,
		Dashboard:   data,
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
		vaults, settings, err := s.service.ListVaults(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.render(w, "vaults", http.StatusOK, TemplateData{
			Title:       "Vaults",
			Notice:      r.URL.Query().Get("notice"),
			AppSettings: settings,
			Vaults:      vaults,
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

		if _, err := s.service.CreateVault(r.Context(), application.CreateVaultInput{
			CharacterName:   r.FormValue("character_name"),
			StrengthScore:   strength,
			CarryModifierLB: carryModifier,
			Notes:           r.FormValue("notes"),
		}); err != nil {
			vaults, settings, listErr := s.service.ListVaults(r.Context())
			if listErr != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			s.render(w, "vaults", http.StatusBadRequest, TemplateData{
				Title:       "Vaults",
				Error:       err.Error(),
				AppSettings: settings,
				Vaults:      vaults,
			})
			return
		}
		http.Redirect(w, r, "/vaults?notice=Vault+created", http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleVaultRoutes(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/purse"):
		s.handleVaultPurse(w, r)
	default:
		s.handleVaultDetail(w, r)
	}
}

func (s *Server) handleVaultDetail(w http.ResponseWriter, r *http.Request) {
	id := pathSegment(r.URL.Path, "/vaults/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

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

		_, err = s.service.UpdateVault(r.Context(), application.UpdateVaultInput{
			ID:              id,
			CharacterName:   r.FormValue("character_name"),
			StrengthScore:   strength,
			CarryModifierLB: carryModifier,
			EncumbranceMode: domain.ParseEncumbranceMode(r.FormValue("encumbrance_mode")),
			Notes:           r.FormValue("notes"),
			Archived:        r.FormValue("archived") == "on",
		})
		if err != nil {
			data, detailErr := s.service.GetVaultDetail(r.Context(), id)
			if detailErr != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			s.render(w, "vault_detail", http.StatusBadRequest, TemplateData{
				Title:       "Vault",
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

	s.render(w, "vault_detail", http.StatusOK, TemplateData{
		Title:       data.Summary.Vault.CharacterName,
		Notice:      r.URL.Query().Get("notice"),
		AppSettings: data.Settings,
		VaultDetail: data,
	})
}

func (s *Server) handleVaultPurse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := pathSegment(strings.TrimSuffix(r.URL.Path, "/purse"), "/vaults/")
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
	items, settings, err := s.service.ListCompendium(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render(w, "compendium", http.StatusOK, TemplateData{
		Title:           "Compendium",
		Notice:          r.URL.Query().Get("notice"),
		AppSettings:     settings,
		CompendiumItems: items,
	})
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
	case strings.HasSuffix(r.URL.Path, "/split"):
		s.handleItemSplit(w, r)
	case strings.HasSuffix(r.URL.Path, "/merge"):
		s.handleItemMerge(w, r)
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
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if _, err := s.service.SaveItem(r.Context(), input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
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
		Categories:                domain.Categories(),
		Rarities:                  domain.Rarities(),
		SelectedLocationKind:      string(detail.Item.Location.Kind),
		SelectedVaultID:           detail.Item.Location.OwnerVaultID,
		SelectedParentContainerID: detail.Item.Location.ParentContainerItemID,
	})
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

	preview := s.service.PreviewBulk(r.FormValue("bulk_text"), domain.BulkDefaults{
		Category:           strings.TrimSpace(r.FormValue("default_category")),
		Rarity:             domain.ParseRarity(r.FormValue("default_rarity")),
		WeightHundredthsLB: weight,
		BaseValueCP:        value,
	})
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
		properties := []string{}
		for _, property := range strings.Split(r.FormValue("weapon_properties"), ",") {
			property = strings.TrimSpace(property)
			if property != "" {
				properties = append(properties, property)
			}
		}
		if weaponClass != "" {
			details.Weapon = &domain.WeaponDetails{
				WeaponClass: weaponClass,
				DamageDice:  r.FormValue("weapon_damage_dice"),
				DamageType:  r.FormValue("weapon_damage_type"),
				Properties:  properties,
				NormalRange: normalRange,
				LongRange:   longRange,
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
