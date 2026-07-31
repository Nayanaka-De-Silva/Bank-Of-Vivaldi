package httpui

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"bank-of-vivaldi/internal/application"
	"bank-of-vivaldi/internal/domain"
)

func TestNormalizeItemFormCategory(t *testing.T) {
	t.Parallel()

	if got := normalizeItemFormCategory(""); got != defaultItemFormCategory {
		t.Fatalf("expected default category %q, got %q", defaultItemFormCategory, got)
	}

	if got := normalizeItemFormCategory(" Weapon "); got != "weapon" {
		t.Fatalf("expected normalized category %q, got %q", "weapon", got)
	}

	if !containerMetadataVisible("weapon", true) {
		t.Fatalf("expected container metadata to remain visible for container items outside the container category")
	}
}

func TestItemFormFieldsDefaultCategorySelection(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	err = server.tmpl.ExecuteTemplate(&rendered, "item_form_fields", TemplateData{
		Categories:           domain.Categories(),
		Rarities:             domain.Rarities(),
		SelectedLocationKind: string(domain.LocationKindCompendiumRoot),
	})
	if err != nil {
		t.Fatalf("render item form fields: %v", err)
	}

	html := rendered.String()
	if !strings.Contains(html, `<option value="equipment" selected>`) {
		t.Fatalf("expected equipment to be selected by default, got:\n%s", html)
	}
	if !strings.Contains(html, `data-metadata-section hidden`) {
		t.Fatalf("expected specialized metadata section to be hidden when no metadata applies, got:\n%s", html)
	}
	if strings.Contains(html, `No specialized metadata fields are needed for the selected category.`) {
		t.Fatalf("expected empty metadata helper copy to be removed, got:\n%s", html)
	}
}

func TestItemFormFieldsShowOnlyMatchingMetadataGroup(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	err = server.tmpl.ExecuteTemplate(&rendered, "item_form_fields", TemplateData{
		Categories:           domain.Categories(),
		Rarities:             domain.Rarities(),
		Item:                 domain.Item{Category: "weapon"},
		SelectedLocationKind: string(domain.LocationKindCompendiumRoot),
	})
	if err != nil {
		t.Fatalf("render item form fields: %v", err)
	}

	html := rendered.String()
	if strings.Contains(html, `data-metadata-section hidden`) {
		t.Fatalf("expected specialized metadata section to be visible for matching metadata, got:\n%s", html)
	}
	if !strings.Contains(html, `data-metadata-group="weapon"`) {
		t.Fatalf("expected weapon metadata group in markup, got:\n%s", html)
	}
	if !strings.Contains(html, `data-metadata-group="armor" hidden`) {
		t.Fatalf("expected non-matching armor metadata group to be hidden, got:\n%s", html)
	}
	if !strings.Contains(html, `activeCategory === "container" || Boolean(isContainerCheckbox && isContainerCheckbox.checked)`) {
		t.Fatalf("expected container toggle logic in metadata visibility script, got:\n%s", html)
	}
	if !strings.Contains(html, `input.disabled = !visible;`) {
		t.Fatalf("expected inactive metadata inputs to be disabled in the visibility script, got:\n%s", html)
	}
	if !strings.Contains(html, `metadataSection.hidden = !hasVisibleGroup;`) {
		t.Fatalf("expected specialized metadata section visibility to track active groups, got:\n%s", html)
	}
}

func TestItemFormFieldsKeepContainerMetadataVisibleForContainerItems(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	err = server.tmpl.ExecuteTemplate(&rendered, "item_form_fields", TemplateData{
		Categories:           domain.Categories(),
		Rarities:             domain.Rarities(),
		Item:                 domain.Item{Category: "weapon", IsContainer: true},
		SelectedLocationKind: string(domain.LocationKindCompendiumRoot),
	})
	if err != nil {
		t.Fatalf("render item form fields: %v", err)
	}

	html := rendered.String()
	if strings.Contains(html, `data-metadata-section hidden`) {
		t.Fatalf("expected specialized metadata section to stay visible for container items, got:\n%s", html)
	}
	if strings.Contains(html, `data-metadata-group="container" hidden`) {
		t.Fatalf("expected container metadata group to stay visible for container items, got:\n%s", html)
	}
}

func TestStylesheetPreservesHiddenAttribute(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("resolve current file path")
	}

	stylesheetPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "web", "static", "styles.css")
	css, err := os.ReadFile(stylesheetPath)
	if err != nil {
		t.Fatalf("read stylesheet: %v", err)
	}

	stylesheet := string(css)
	if !strings.Contains(stylesheet, "[hidden] {") {
		t.Fatalf("expected stylesheet to define a hidden attribute rule")
	}
	if !strings.Contains(stylesheet, "display: none !important;") {
		t.Fatalf("expected hidden attribute rule to force display none")
	}
}

// readStylesheet loads web/static/styles.css relative to this test file.
func readStylesheet(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("resolve current file path")
	}

	stylesheetPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "web", "static", "styles.css")
	css, err := os.ReadFile(stylesheetPath)
	if err != nil {
		t.Fatalf("read stylesheet: %v", err)
	}
	return string(css)
}

func TestStylesheetClampsCompendiumCardDescription(t *testing.T) {
	t.Parallel()

	stylesheet := readStylesheet(t)
	for _, declaration := range []string{
		".item-card__desc {",
		"-webkit-line-clamp: 3;",
		"-webkit-box-orient: vertical;",
		"overflow: hidden;",
	} {
		if !strings.Contains(stylesheet, declaration) {
			t.Fatalf("expected card descriptions to be clamped via %q", declaration)
		}
	}
}

func TestStylesheetStretchesCompendiumCardLink(t *testing.T) {
	t.Parallel()

	stylesheet := readStylesheet(t)
	for _, declaration := range []string{
		".item-card__title a::after {",
		"position: absolute;",
		"inset: 0;",
	} {
		if !strings.Contains(stylesheet, declaration) {
			t.Fatalf("expected the card title link to be stretched over the tile via %q", declaration)
		}
	}

	card := strings.Index(stylesheet, ".item-card {")
	if card < 0 {
		t.Fatalf("expected an .item-card rule in the stylesheet")
	}
	cardRule := stylesheet[card:]
	if end := strings.Index(cardRule, "}"); end >= 0 {
		cardRule = cardRule[:end]
	}
	if !strings.Contains(cardRule, "position: relative;") {
		t.Fatalf("expected .item-card to establish a positioning context, got:\n%s", cardRule)
	}
}

func TestParseItemDetailsIgnoresInactiveCategoryMetadata(t *testing.T) {
	t.Parallel()

	form := url.Values{
		"armor_category":          {"medium"},
		"armor_base_ac":           {"14"},
		"weapon_class":            {"martial"},
		"weapon_damage_dice":      {"1d8"},
		"weapon_damage_type":      {"slashing"},
		"weapon_properties":       {"versatile"},
		"weapon_normal_range":     {"20"},
		"weapon_long_range":       {"60"},
		"container_max_weight_lb": {"10"},
	}

	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("parse form: %v", err)
	}

	details, err := parseItemDetails(req, "armor", false)
	if err != nil {
		t.Fatalf("parse item details: %v", err)
	}

	if details.Armor == nil {
		t.Fatalf("expected armor metadata to be parsed")
	}
	if details.Weapon != nil {
		t.Fatalf("expected weapon metadata to be ignored when category is armor")
	}
	if details.Container != nil {
		t.Fatalf("expected container metadata to be ignored when item is not a container")
	}
}

func TestParseItemDetailsKeepsContainerMetadataForContainerItems(t *testing.T) {
	t.Parallel()

	form := url.Values{
		"container_max_weight_lb": {"15"},
	}

	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("parse form: %v", err)
	}

	details, err := parseItemDetails(req, "weapon", true)
	if err != nil {
		t.Fatalf("parse item details: %v", err)
	}

	if details.Container == nil {
		t.Fatalf("expected container metadata to be preserved for container items")
	}
}

func TestParseItemDetailsParsesValidWeaponCategory(t *testing.T) {
	t.Parallel()

	form := url.Values{
		"weapon_class":        {"martial-ranged"},
		"weapon_damage_dice":  {"1d8"},
		"weapon_damage_type":  {"piercing"},
		"weapon_properties":   {"ammunition, heavy, two-handed"},
		"weapon_normal_range": {"150"},
		"weapon_long_range":   {"600"},
	}

	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("parse form: %v", err)
	}

	details, err := parseItemDetails(req, "weapon", false)
	if err != nil {
		t.Fatalf("parse item details: %v", err)
	}

	if details.Weapon == nil {
		t.Fatalf("expected weapon metadata to be parsed for a valid weapon category")
	}
	if details.Weapon.WeaponClass != "martial-ranged" {
		t.Fatalf("weapon class = %q, want %q", details.Weapon.WeaponClass, "martial-ranged")
	}
	if details.Weapon.NormalRange != 150 || details.Weapon.LongRange != 600 {
		t.Fatalf("weapon ranges = %d/%d, want 150/600", details.Weapon.NormalRange, details.Weapon.LongRange)
	}
}

func TestParseItemDetailsRejectsInvalidWeaponCategory(t *testing.T) {
	t.Parallel()

	// "martial" is not one of the four allowed categories; the dropdown can be
	// bypassed, so the server must reject it.
	form := url.Values{
		"weapon_class": {"martial"},
	}

	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("parse form: %v", err)
	}

	if _, err := parseItemDetails(req, "weapon", false); err == nil {
		t.Fatalf("expected an error for an invalid weapon category")
	}
}

func TestParseItemDetailsAllowsEmptyWeaponCategory(t *testing.T) {
	t.Parallel()

	// An empty weapon category means "not set" and must simply produce no weapon details.
	form := url.Values{
		"weapon_class": {""},
	}

	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("parse form: %v", err)
	}

	details, err := parseItemDetails(req, "weapon", false)
	if err != nil {
		t.Fatalf("parse item details: %v", err)
	}
	if details.Weapon != nil {
		t.Fatalf("expected no weapon metadata when the category is empty")
	}
}

func TestItemFormLocationCompendiumKindDisablesBothSelects(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	err = server.tmpl.ExecuteTemplate(&rendered, "item_form_fields", TemplateData{
		Categories:           domain.Categories(),
		Rarities:             domain.Rarities(),
		SelectedLocationKind: string(domain.LocationKindCompendiumRoot),
	})
	if err != nil {
		t.Fatalf("render item form fields: %v", err)
	}

	html := rendered.String()
	if !strings.Contains(html, `name="vault_id" disabled`) {
		t.Fatalf("expected vault_id select to be disabled for compendium location kind, got:\n%s", html)
	}
	if !strings.Contains(html, `name="parent_container_item_id" disabled`) {
		t.Fatalf("expected parent_container_item_id select to be disabled for compendium location kind, got:\n%s", html)
	}
}

func TestItemFormLocationVaultKindDisablesOnlyContainerSelect(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	err = server.tmpl.ExecuteTemplate(&rendered, "item_form_fields", TemplateData{
		Categories:           domain.Categories(),
		Rarities:             domain.Rarities(),
		SelectedLocationKind: string(domain.LocationKindVaultRoot),
		AllVaults:            []domain.Vault{{ID: "vault-1", CharacterName: "Aragorn"}},
		SelectedVaultID:      "vault-1",
	})
	if err != nil {
		t.Fatalf("render item form fields: %v", err)
	}

	html := rendered.String()
	if strings.Contains(html, `name="vault_id" disabled`) {
		t.Fatalf("expected vault_id select to be enabled for vault location kind, got:\n%s", html)
	}
	if !strings.Contains(html, `name="parent_container_item_id" disabled`) {
		t.Fatalf("expected parent_container_item_id select to be disabled for vault location kind, got:\n%s", html)
	}
}

func TestItemFormLocationContainerKindFiltersByVault(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	err = server.tmpl.ExecuteTemplate(&rendered, "item_form_fields", TemplateData{
		Categories:           domain.Categories(),
		Rarities:             domain.Rarities(),
		SelectedLocationKind: string(domain.LocationKindContainer),
		SelectedVaultID:      "vault-1",
		AllVaults: []domain.Vault{
			{ID: "vault-1", CharacterName: "Aragorn"},
			{ID: "vault-2", CharacterName: "Legolas"},
		},
		ContainerOptions: []domain.Item{
			{ID: "c1", Name: "Chest", Location: domain.ItemLocation{OwnerVaultID: "vault-1"}},
			{ID: "c2", Name: "Bag", Location: domain.ItemLocation{OwnerVaultID: "vault-2"}},
		},
	})
	if err != nil {
		t.Fatalf("render item form fields: %v", err)
	}

	html := rendered.String()
	if !strings.Contains(html, `data-vault-id="vault-1"`) {
		t.Fatalf("expected container options to carry data-vault-id attribute, got:\n%s", html)
	}
	if !strings.Contains(html, `data-vault-id="vault-2"`) {
		t.Fatalf("expected all container options to carry data-vault-id attribute, got:\n%s", html)
	}
	// Matching-vault container (vault-1) must not be hidden.
	if strings.Contains(html, `data-vault-id="vault-1" hidden`) {
		t.Fatalf("expected vault-1 container option to be visible, got:\n%s", html)
	}
	// Non-matching container (vault-2) must be hidden and disabled.
	if !strings.Contains(html, `data-vault-id="vault-2" hidden disabled`) {
		t.Fatalf("expected non-matching vault-2 container option to be hidden and disabled, got:\n%s", html)
	}
}

func TestItemFormLocationScriptContainsSyncLogic(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	err = server.tmpl.ExecuteTemplate(&rendered, "item_form_fields", TemplateData{
		Categories:           domain.Categories(),
		Rarities:             domain.Rarities(),
		SelectedLocationKind: string(domain.LocationKindCompendiumRoot),
	})
	if err != nil {
		t.Fatalf("render item form fields: %v", err)
	}

	html := rendered.String()
	if !strings.Contains(html, `option.dataset.vaultId`) {
		t.Fatalf("expected location sync script to reference option.dataset.vaultId, got:\n%s", html)
	}
	if !strings.Contains(html, `syncLocationFields`) {
		t.Fatalf("expected location sync script to define syncLocationFields, got:\n%s", html)
	}
	if !strings.Contains(html, `locationKindSelect.addEventListener`) {
		t.Fatalf("expected location sync script to listen on location_kind changes, got:\n%s", html)
	}
	if !strings.Contains(html, `vaultSelect.addEventListener`) {
		t.Fatalf("expected location sync script to listen on vault_id changes, got:\n%s", html)
	}
}

func TestItemFormWeaponCategoryRendersFourOptions(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	err = server.tmpl.ExecuteTemplate(&rendered, "item_form_fields", TemplateData{
		Categories: domain.Categories(),
		Rarities:   domain.Rarities(),
	})
	if err != nil {
		t.Fatalf("render item form fields: %v", err)
	}

	html := rendered.String()
	if !strings.Contains(html, `<select name="weapon_class">`) {
		t.Fatalf("expected weapon category to render as a select, got:\n%s", html)
	}
	for _, category := range domain.WeaponCategories() {
		option := `<option value="` + category + `"`
		if !strings.Contains(html, option) {
			t.Fatalf("expected weapon category option %q to be rendered, got:\n%s", category, html)
		}
		if label := domain.HumanizeLabel(category); !strings.Contains(html, label) {
			t.Fatalf("expected weapon category label %q to be rendered, got:\n%s", label, html)
		}
	}
	if !strings.Contains(html, `data-weapon-range-fields`) {
		t.Fatalf("expected a toggleable range-fields container, got:\n%s", html)
	}
	if !strings.Contains(html, `weaponClassSelect.addEventListener`) {
		t.Fatalf("expected weapon range script to listen on weapon category changes, got:\n%s", html)
	}
}

func TestItemFormWeaponCategoryMarksSavedSelection(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	item := domain.Item{Category: "weapon"}
	item.Details.Weapon = &domain.WeaponDetails{WeaponClass: "martial-ranged"}

	var rendered bytes.Buffer
	err = server.tmpl.ExecuteTemplate(&rendered, "item_form_fields", TemplateData{
		Categories: domain.Categories(),
		Rarities:   domain.Rarities(),
		Item:       item,
	})
	if err != nil {
		t.Fatalf("render item form fields: %v", err)
	}

	html := rendered.String()
	if !strings.Contains(html, `<option value="martial-ranged" selected>`) {
		t.Fatalf("expected the saved weapon category to be pre-selected, got:\n%s", html)
	}
}

func compendiumTemplateData(view string, filters application.CompendiumFilters) TemplateData {
	scaleMail := domain.Item{
		ID:                 "scale",
		Name:               "Scale Mail",
		Category:           "armor",
		Rarity:             domain.RarityRare,
		Description:        "Interlocking metal rings sewn onto a leather backing.",
		WeightHundredthsLB: 4500,
		BaseValueCP:        5000,
		Quantity:           1,
	}
	scaleMail.Details.Armor = &domain.ArmorDetails{ArmorCategory: "medium", BaseAC: 14}

	return TemplateData{
		Title:             "Compendium",
		Categories:        domain.Categories(),
		Rarities:          domain.Rarities(),
		CompendiumView:    view,
		CompendiumFilters: filters,
		Compendium: application.CompendiumBrowse{
			Entries: []application.CompendiumEntry{{
				Item:           scaleMail,
				ContainerPath:  "Oak Chest",
				IsNested:       true,
				UnitWeightLB:   "45",
				UnitValueText:  "50 gp",
				TotalWeightLB:  "45",
				TotalValueText: "50 gp",
			}},
			Facets:     domain.CompendiumFacets{ArmorCategories: []string{"medium"}},
			TotalCount: 3,
			MatchCount: 1,
		},
	}
}

func renderCompendium(t *testing.T, data TemplateData) string {
	t.Helper()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	if err := server.tmpl.ExecuteTemplate(&rendered, "compendium", data); err != nil {
		t.Fatalf("render compendium: %v", err)
	}
	return rendered.String()
}

func TestCompendiumRendersTilesByDefault(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{}))

	if !strings.Contains(html, `class="card-grid"`) {
		t.Fatalf("expected a card grid in the tile view, got:\n%s", html)
	}
	if !strings.Contains(html, `class="item-card`) {
		t.Fatalf("expected item cards in the tile view, got:\n%s", html)
	}
	if strings.Contains(html, "<table") {
		t.Fatalf("expected no table in the tile view, got:\n%s", html)
	}
	for _, fragment := range []string{"Scale Mail", "Armor", "Rare", "45 lb", "50 gp", "Interlocking metal rings"} {
		if !strings.Contains(html, fragment) {
			t.Fatalf("expected card to show %q, got:\n%s", fragment, html)
		}
	}
}

func TestCompendiumCardOrdersFieldsPerIssueLayout(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{}))

	name := strings.Index(html, `class="item-card__title"`)
	meta := strings.Index(html, `class="item-card__meta"`)
	stats := strings.Index(html, `class="item-card__stats"`)
	// Anchored on the element, not the text: the description also appears in the card tooltip.
	description := strings.Index(html, `class="item-card__desc"`)

	if name < 0 || meta < 0 || stats < 0 || description < 0 {
		t.Fatalf("expected name, meta, stats and description on the card, got:\n%s", html)
	}
	if !(name < meta && meta < stats && stats < description) {
		t.Fatalf("expected card order name -> category/rarity -> weight/value -> description, got:\n%s", html)
	}
	if !strings.Contains(html, ">Scale Mail<") {
		t.Fatalf("expected the card title to show the item name, got:\n%s", html)
	}
}

func TestCompendiumCardExposesFullDescriptionAsTooltip(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{}))

	tooltip := `title="Interlocking metal rings sewn onto a leather backing."`
	card := strings.Index(html, `<article class="item-card`)
	if card < 0 {
		t.Fatalf("expected an item card article, got:\n%s", html)
	}
	openingTag := html[card:]
	if end := strings.Index(openingTag, ">"); end >= 0 {
		openingTag = openingTag[:end]
	}
	if !strings.Contains(openingTag, tooltip) {
		t.Fatalf("expected the card to carry the full description as a tooltip, got:\n%s", openingTag)
	}

	// The stretched link relies on the card holding exactly one anchor.
	if count := strings.Count(html[card:], `<a href="/items/`); count != 1 {
		t.Fatalf("expected exactly one item link per card, got %d:\n%s", count, html)
	}
}

func TestCompendiumCardOmitsTooltipWithoutDescription(t *testing.T) {
	t.Parallel()

	data := compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{})
	data.Compendium.Entries[0].Item.Description = ""

	html := renderCompendium(t, data)

	if strings.Contains(html, `title=""`) {
		t.Fatalf("expected no empty tooltip when the item has no description, got:\n%s", html)
	}
	if strings.Contains(html, `class="item-card__desc"`) {
		t.Fatalf("expected no description paragraph when the item has no description, got:\n%s", html)
	}
}

func TestCompendiumListViewRendersTable(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, compendiumTemplateData(compendiumViewList, application.CompendiumFilters{}))

	if !strings.Contains(html, "<table") {
		t.Fatalf("expected a table in the list view, got:\n%s", html)
	}
	if strings.Contains(html, `class="card-grid"`) {
		t.Fatalf("expected no card grid in the list view, got:\n%s", html)
	}
	if !strings.Contains(html, "Container path") {
		t.Fatalf("expected the list view to show the container path column, got:\n%s", html)
	}
}

func TestCompendiumViewToggleSubmitsWithinTheFilterForm(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{Query: "mail"}))

	if !strings.Contains(html, `name="view" value="list"`) {
		t.Fatalf("expected a list-view submit button, got:\n%s", html)
	}
	if !strings.Contains(html, `name="view" value="tiles"`) {
		t.Fatalf("expected a tile-view submit button, got:\n%s", html)
	}
	// The active filters must ride along with the toggle, so they live in the same form.
	if !strings.Contains(html, `name="q" value="mail"`) {
		t.Fatalf("expected active filters to be repopulated in the form, got:\n%s", html)
	}
	if strings.Contains(html, `type="hidden" name="view"`) {
		t.Fatalf("expected no hidden view input; it would win over the toggle buttons, got:\n%s", html)
	}
}

func TestCompendiumFilterGroupsFollowSelectedCategory(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{}))
	for _, group := range []string{"armor", "weapon", "container", "tool", "mount", "vehicle", "treasure"} {
		if !strings.Contains(html, `data-filter-group="`+group+`" hidden`) {
			t.Fatalf("expected the %s filter group to be hidden with no category selected, got:\n%s", group, html)
		}
	}

	html = renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{Category: "armor"}))
	if strings.Contains(html, `data-filter-group="armor" hidden`) {
		t.Fatalf("expected the armor filter group to be visible for the armor category, got:\n%s", html)
	}
	if !strings.Contains(html, `data-filter-group="weapon" hidden`) {
		t.Fatalf("expected the weapon filter group to stay hidden for the armor category, got:\n%s", html)
	}
	if !strings.Contains(html, `name="armor_min_ac"`) || !strings.Contains(html, `name="armor_max_ac"`) {
		t.Fatalf("expected armor AC range filters, got:\n%s", html)
	}
	// The armor category dropdown is built from the values present in the data.
	if !strings.Contains(html, `<option value="medium"`) {
		t.Fatalf("expected armor category facet options, got:\n%s", html)
	}
}

func TestCompendiumWeaponGroupUsesTheFourWeaponCategories(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{Category: "weapon"}))

	if !strings.Contains(html, `name="weapon_category"`) {
		t.Fatalf("expected a weapon category filter, got:\n%s", html)
	}
	for _, category := range domain.WeaponCategories() {
		if !strings.Contains(html, `<option value="`+category+`"`) {
			t.Fatalf("expected weapon category option %q, got:\n%s", category, html)
		}
	}
}

func TestCompendiumFilterScriptDisablesHiddenGroupInputs(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{}))

	if !strings.Contains(html, `input.disabled = !visible;`) {
		t.Fatalf("expected hidden filter group inputs to be disabled so they never reach the query string, got:\n%s", html)
	}
	if !strings.Contains(html, `categorySelect.addEventListener`) {
		t.Fatalf("expected the filter script to react to category changes, got:\n%s", html)
	}

	// The script anchors itself with document.currentScript.closest("form"), so it
	// must live inside the filter form or it silently does nothing.
	formStart := strings.Index(html, "data-compendium-filters")
	if formStart < 0 {
		t.Fatalf("expected the compendium filter form, got:\n%s", html)
	}
	rest := html[formStart:]
	scriptStart := strings.Index(rest, `document.currentScript.closest("form")`)
	formEnd := strings.Index(rest, "</form>")
	if scriptStart < 0 || formEnd < 0 || scriptStart > formEnd {
		t.Fatalf("expected the filter script to sit inside the filter form, got:\n%s", html)
	}
}

func TestCompendiumShowsMatchCounts(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{Category: "armor"}))

	if !strings.Contains(html, "1 of 3") {
		t.Fatalf("expected the matched/total item counts to be shown, got:\n%s", html)
	}
}

func TestCompendiumEmptyResultExplainsTheFilter(t *testing.T) {
	t.Parallel()

	data := compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{Category: "weapon"})
	data.Compendium.Entries = nil
	data.Compendium.MatchCount = 0

	html := renderCompendium(t, data)
	if !strings.Contains(html, "No compendium items match") {
		t.Fatalf("expected an empty-result message, got:\n%s", html)
	}
}

func TestNormalizeCompendiumView(t *testing.T) {
	t.Parallel()

	if got := normalizeCompendiumView(""); got != compendiumViewTiles {
		t.Fatalf("default view = %q, want %q", got, compendiumViewTiles)
	}
	if got := normalizeCompendiumView("list"); got != compendiumViewList {
		t.Fatalf("view = %q, want %q", got, compendiumViewList)
	}
	if got := normalizeCompendiumView("nonsense"); got != compendiumViewTiles {
		t.Fatalf("unknown view = %q, want the tiles default", got)
	}
}

func TestParseCompendiumFiltersReadsEveryQueryParam(t *testing.T) {
	t.Parallel()

	query := url.Values{
		"q":                         {"mail"},
		"category":                  {"armor"},
		"rarity":                    {"rare"},
		"min_weight_lb":             {"1"},
		"max_weight_lb":             {"50"},
		"min_value_gp":              {"10"},
		"max_value_gp":              {"500"},
		"magical":                   {"yes"},
		"attunement":                {"no"},
		"sort":                      {"weight"},
		"armor_category":            {"medium"},
		"armor_dex_behavior":        {"max-2"},
		"armor_min_ac":              {"12"},
		"armor_max_ac":              {"18"},
		"armor_stealth":             {"no"},
		"weapon_category":           {"martial-ranged"},
		"weapon_damage_type":        {"piercing"},
		"weapon_property":           {"heavy"},
		"container_min_capacity_lb": {"5"},
		"container_max_capacity_lb": {"300"},
		"tool_category":             {"thieves"},
		"mount_type":                {"horse"},
		"mount_min_speed":           {"30"},
		"mount_max_speed":           {"80"},
		"vehicle_type":              {"water"},
		"vehicle_min_speed":         {"5"},
		"vehicle_max_speed":         {"25"},
		"treasure_kind":             {"gem"},
	}

	req := httptest.NewRequest(http.MethodGet, "/compendium?"+query.Encode(), nil)
	filters := parseCompendiumFilters(req)

	want := application.CompendiumFilters{
		Query:                  "mail",
		Category:               "armor",
		Rarity:                 "rare",
		MinWeightLB:            "1",
		MaxWeightLB:            "50",
		MinValueGP:             "10",
		MaxValueGP:             "500",
		Magical:                "yes",
		Attunement:             "no",
		SortBy:                 "weight",
		ArmorCategory:          "medium",
		ArmorDexBehavior:       "max-2",
		ArmorMinAC:             "12",
		ArmorMaxAC:             "18",
		ArmorStealth:           "no",
		WeaponCategory:         "martial-ranged",
		WeaponDamageType:       "piercing",
		WeaponProperty:         "heavy",
		ContainerMinCapacityLB: "5",
		ContainerMaxCapacityLB: "300",
		ToolCategory:           "thieves",
		MountType:              "horse",
		MountMinSpeed:          "30",
		MountMaxSpeed:          "80",
		VehicleType:            "water",
		VehicleMinSpeed:        "5",
		VehicleMaxSpeed:        "25",
		TreasureKind:           "gem",
	}

	if filters != want {
		t.Fatalf("filters = %+v, want %+v", filters, want)
	}
}

func TestRarityClass(t *testing.T) {
	t.Parallel()

	if got := rarityClass(domain.RarityVeryRare); got != "rarity-very-rare" {
		t.Fatalf("rarity class = %q, want %q", got, "rarity-very-rare")
	}
}
