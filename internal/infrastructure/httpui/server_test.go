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
