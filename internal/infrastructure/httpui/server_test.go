package httpui

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	if strings.Contains(html, `data-metadata-group="container" hidden`) {
		t.Fatalf("expected container metadata group to stay visible for container items, got:\n%s", html)
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
