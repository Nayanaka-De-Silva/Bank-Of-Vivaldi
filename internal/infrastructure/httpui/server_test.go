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

// readTailwindInput loads web/tailwind/input.css relative to this test file.
// This is the hand-authored source of truth: stable, developer-written CSS
// that survives a Tailwind regeneration, unlike the generated output below.
func readTailwindInput(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("resolve current file path")
	}

	inputPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "web", "tailwind", "input.css")
	css, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read tailwind input: %v", err)
	}
	return string(css)
}

// readGeneratedStylesheet loads the committed, generated web/static/app.css
// relative to this test file.
func readGeneratedStylesheet(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("resolve current file path")
	}

	stylesheetPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "web", "static", "app.css")
	css, err := os.ReadFile(stylesheetPath)
	if err != nil {
		t.Fatalf("read generated stylesheet: %v", err)
	}
	return string(css)
}

func TestTailwindInputPreservesHiddenAttribute(t *testing.T) {
	t.Parallel()

	input := readTailwindInput(t)
	if !strings.Contains(input, "[hidden] {") {
		t.Fatalf("expected the tailwind input to define a hidden attribute rule")
	}
	if !strings.Contains(input, "display: none !important;") {
		t.Fatalf("expected the hidden attribute rule to force display none")
	}
}

func TestTailwindInputKeepsRarityAccentVariables(t *testing.T) {
	t.Parallel()

	input := readTailwindInput(t)
	// Every canonical rarity must set --rarity, or a new rarity added to the
	// domain vocabulary would silently render with no accent colour.
	for _, rarity := range domain.Rarities() {
		class := "." + rarityClass(rarity) + " {"
		if !strings.Contains(input, class) {
			t.Fatalf("expected the tailwind input to define %q for rarity %q", class, rarity)
		}
	}
	if !strings.Contains(input, ".rarity-accent {") || !strings.Contains(input, "var(--rarity") {
		t.Fatalf("expected .rarity-accent to read the --rarity custom property, got:\n%s", input)
	}
}

func TestTailwindInputKeepsPropChipTooltip(t *testing.T) {
	t.Parallel()

	input := readTailwindInput(t)
	for _, fragment := range []string{
		".prop-chip {",
		".prop-chip__tip {",
		"opacity: 0;",
		".prop-chip:hover .prop-chip__tip,",
		".prop-chip:focus-within .prop-chip__tip {",
		"opacity: 1;",
	} {
		if !strings.Contains(input, fragment) {
			t.Fatalf("expected the tailwind input to contain %q, got:\n%s", fragment, input)
		}
	}
}

func TestTailwindInputKeepsNonModalDialogFallback(t *testing.T) {
	t.Parallel()

	input := readTailwindInput(t)
	for _, fragment := range []string{
		"dialog:not(:modal) {",
		"position: static;",
	} {
		if !strings.Contains(input, fragment) {
			t.Fatalf("expected the tailwind input to contain %q, got:\n%s", fragment, input)
		}
	}
}

func TestTailwindInputScansOnlyTheTemplateDirectory(t *testing.T) {
	t.Parallel()

	input := readTailwindInput(t)
	if !strings.Contains(input, `@import "tailwindcss" source(none);`) {
		t.Fatalf("expected automatic content detection to be disabled, got:\n%s", input)
	}
	if !strings.Contains(input, `@source "../templates";`) {
		t.Fatalf("expected an explicit content source rooted at the templates directory, got:\n%s", input)
	}
}

// TestGeneratedStylesheetIsCommitted smoke-tests the build artifact without
// asserting on Tailwind's own output format: a missing, empty, or
// layer-stripped `make css` run would silently ship a bare page.
func TestGeneratedStylesheetIsCommitted(t *testing.T) {
	t.Parallel()

	stylesheet := readGeneratedStylesheet(t)
	if len(stylesheet) < 10000 {
		t.Fatalf("expected the generated stylesheet to be substantial, got %d bytes", len(stylesheet))
	}
	for _, fragment := range []string{"--rarity", ".prop-chip__tip", "[hidden]"} {
		if !strings.Contains(stylesheet, fragment) {
			t.Fatalf("expected the generated stylesheet to contain %q", fragment)
		}
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

	// Checkboxes submit repeated values, not a single comma-joined string.
	form := url.Values{
		"weapon_class":        {"martial-ranged"},
		"weapon_damage_dice":  {"1d8"},
		"weapon_damage_type":  {"piercing"},
		"weapon_properties":   {"ammunition", "heavy", "two-handed"},
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
	// All three submitted properties must be stored.
	wantProps := []string{"ammunition", "heavy", "two-handed"}
	if !stringSlicesEqual(details.Weapon.Properties, wantProps) {
		t.Fatalf("weapon properties = %v, want %v", details.Weapon.Properties, wantProps)
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

func TestParseItemDetailsRepeatedWeaponPropertiesParseIntoSlice(t *testing.T) {
	t.Parallel()

	// Repeated form values (checkbox semantics) must be read via r.Form, not r.FormValue.
	form := url.Values{
		"weapon_class":      {"simple-melee"},
		"weapon_properties": {"finesse", "light"},
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
		t.Fatalf("expected weapon metadata")
	}
	want := []string{"finesse", "light"}
	if !stringSlicesEqual(details.Weapon.Properties, want) {
		t.Fatalf("properties = %v, want %v", details.Weapon.Properties, want)
	}
}

func TestParseItemDetailsCanonicalizesKnownPropertyVariants(t *testing.T) {
	t.Parallel()

	// "Two-Handed" (form variant) must be stored as the canonical slug "two-handed".
	form := url.Values{
		"weapon_class":      {"martial-melee"},
		"weapon_properties": {"Two-Handed"},
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
		t.Fatalf("expected weapon metadata")
	}
	if !stringSlicesEqual(details.Weapon.Properties, []string{"two-handed"}) {
		t.Fatalf("properties = %v, want [two-handed]", details.Weapon.Properties)
	}
}

func TestParseItemDetailsPreservesUnknownPropertyValues(t *testing.T) {
	t.Parallel()

	// "glowing" is not a canonical PHB property; it must be kept as-is with no error.
	form := url.Values{
		"weapon_class":      {"martial-melee"},
		"weapon_properties": {"glowing"},
	}

	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("parse form: %v", err)
	}

	details, err := parseItemDetails(req, "weapon", false)
	if err != nil {
		t.Fatalf("expected no error for unknown property, got: %v", err)
	}
	if details.Weapon == nil {
		t.Fatalf("expected weapon metadata")
	}
	if !stringSlicesEqual(details.Weapon.Properties, []string{"glowing"}) {
		t.Fatalf("properties = %v, want [glowing]", details.Weapon.Properties)
	}
}

func TestParseItemDetailsPropertiesSurviveWithEmptyWeaponClass(t *testing.T) {
	t.Parallel()

	// Properties must be stored even when weapon_class is not set.
	form := url.Values{
		"weapon_class":      {""},
		"weapon_properties": {"versatile"},
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
		t.Fatalf("expected weapon metadata to be set when properties are present even without a weapon class")
	}
	if !stringSlicesEqual(details.Weapon.Properties, []string{"versatile"}) {
		t.Fatalf("properties = %v, want [versatile]", details.Weapon.Properties)
	}
}

func TestParseItemDetailsParsesVersatileDamageDice(t *testing.T) {
	t.Parallel()

	// A checked versatile property alongside a versatile die must be stored on WeaponDetails.
	form := url.Values{
		"weapon_properties":            {"versatile"},
		"weapon_versatile_damage_dice": {"1d8"},
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
		t.Fatalf("expected weapon metadata to be set when versatile property is present")
	}
	if details.Weapon.VersatileDamageDice != "1d8" {
		t.Fatalf("VersatileDamageDice = %q, want %q", details.Weapon.VersatileDamageDice, "1d8")
	}
}

// stringSlicesEqual reports whether two string slices are element-wise equal.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

func TestLocationFieldsPartialRendersTheThreeSelects(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	err = server.tmpl.ExecuteTemplate(&rendered, "location_fields", TemplateData{
		SelectedLocationKind: string(domain.LocationKindContainer),
		SelectedVaultID:      "vault-1",
		AllVaults: []domain.Vault{
			{ID: "vault-1", CharacterName: "Aragorn"},
		},
		ContainerOptions: []domain.Item{
			{ID: "c1", Name: "Chest", Location: domain.ItemLocation{OwnerVaultID: "vault-1"}},
		},
	})
	if err != nil {
		t.Fatalf("render location_fields: %v", err)
	}

	html := rendered.String()
	for _, want := range []string{
		`name="location_kind"`,
		`name="vault_id"`,
		`name="parent_container_item_id"`,
		`data-vault-id="vault-1"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected location_fields to contain %q, got:\n%s", want, html)
		}
	}
}

func renderItemDetailFromItem(t *testing.T, item domain.Item) string {
	t.Helper()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	err = server.tmpl.ExecuteTemplate(&rendered, "item_detail", TemplateData{
		ItemDetail:           application.ItemDetail{Item: item},
		Item:                 item,
		Categories:           domain.Categories(),
		Rarities:             domain.Rarities(),
		SelectedLocationKind: string(item.Location.Kind),
		SelectedVaultID:      item.Location.OwnerVaultID,
	})
	if err != nil {
		t.Fatalf("render item detail: %v", err)
	}
	return rendered.String()
}

func TestItemDetailRendersMoveCopyAndDeleteDialogs(t *testing.T) {
	t.Parallel()

	html := renderItemDetailFromItem(t, domain.Item{ID: "item-1", Name: "Rope"})
	for _, want := range []string{`id="move-dialog"`, `id="copy-dialog"`, `id="delete-dialog"`, "<dialog"} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected item detail to render %q, got:\n%s", want, html)
		}
	}
}

func TestItemCopyDialogPostsToCopyRoute(t *testing.T) {
	t.Parallel()

	html := renderItemDetailFromItem(t, domain.Item{ID: "item-1", Name: "Rope"})
	if !strings.Contains(html, `action="/items/item-1/copy"`) {
		t.Fatalf("expected copy dialog to post to /items/item-1/copy, got:\n%s", html)
	}
}

func TestItemMoveDialogStillPostsToMoveRoute(t *testing.T) {
	t.Parallel()

	html := renderItemDetailFromItem(t, domain.Item{ID: "item-1", Name: "Rope"})
	if !strings.Contains(html, `action="/items/item-1/move"`) {
		t.Fatalf("expected move dialog to post to /items/item-1/move, got:\n%s", html)
	}
}

func TestItemDeleteDialogReplacesNativeConfirm(t *testing.T) {
	t.Parallel()

	html := renderItemDetailFromItem(t, domain.Item{ID: "item-1", Name: "Rope"})
	if !strings.Contains(html, `id="delete-dialog"`) {
		t.Fatalf("expected a delete dialog, got:\n%s", html)
	}
	if strings.Contains(html, "confirm(") {
		t.Fatalf("expected native confirm() to be replaced by the delete dialog, got:\n%s", html)
	}
}

func TestItemDetailReusesLocationFieldsInBothDialogs(t *testing.T) {
	t.Parallel()

	html := renderItemDetailFromItem(t, domain.Item{ID: "item-1", Name: "Rope"})
	// Edit form + move dialog + copy dialog: proof the picker is a shared
	// partial rather than pasted markup drifting out of sync. Matching the
	// opening tag (not the bare attribute) avoids also counting the JS
	// selector string 'select[name="location_kind"]' in each copy's script.
	if got := strings.Count(html, `<select name="location_kind">`); got != 3 {
		t.Fatalf("expected location_kind select to appear 3 times, got %d in:\n%s", got, html)
	}
}

func TestItemDialogsRemainUsableWithoutScript(t *testing.T) {
	t.Parallel()

	html := renderItemDetailFromItem(t, domain.Item{ID: "item-1", Name: "Rope"})
	for _, dialog := range []string{"move-dialog", "copy-dialog", "delete-dialog"} {
		if !strings.Contains(html, `id="`+dialog+`" aria-labelledby="`+dialog+`-title" open`) {
			t.Fatalf("expected %s to render with the open attribute for the no-JS fallback, got:\n%s", dialog, html)
		}
	}
	if !strings.Contains(html, "dialog.close()") {
		t.Fatalf("expected dialog script to close dialogs once scripting is confirmed, got:\n%s", html)
	}
	if !strings.Contains(html, "trigger.hidden = false") {
		t.Fatalf("expected dialog script to reveal triggers once scripting is confirmed, got:\n%s", html)
	}
}

func TestItemDialogScriptWiresModalBehaviour(t *testing.T) {
	t.Parallel()

	html := renderItemDetailFromItem(t, domain.Item{ID: "item-1", Name: "Rope"})
	for _, want := range []string{
		"showModal()",
		"data-dialog-open",
		"data-dialog-close",
		"event.target === dialog",
		"trigger.focus()",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected dialog script to contain %q, got:\n%s", want, html)
		}
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

	browse := application.CompendiumBrowse{
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
	}

	return TemplateData{
		Title:             "Compendium",
		Categories:        domain.Categories(),
		Rarities:          domain.Rarities(),
		CompendiumView:    view,
		CompendiumFilters: filters,
		Compendium:        browse,
		FilterBar:         newFilterBar("/compendium", view, compendiumViews, filters, browse),
	}
}

// renderPartial executes a named template directly against an arbitrary dot,
// bypassing the page wrapper. Used to prove the item_browse.html partials
// (issue #18) generalize beyond the compendium's own TemplateData shape --
// the vault browser (added in a later commit) drives them with the same
// FilterBarData/[]ItemEntry types but different underlying data.
func renderPartial(t *testing.T, name string, dot any) string {
	t.Helper()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	if err := server.tmpl.ExecuteTemplate(&rendered, name, dot); err != nil {
		t.Fatalf("render %s: %v", name, err)
	}
	return rendered.String()
}

func TestItemFilterBarIsSharedByCompendiumAndVault(t *testing.T) {
	t.Parallel()

	compendiumBar := newFilterBar("/compendium", compendiumViewTiles, compendiumViews, application.CompendiumFilters{Query: "torch"}, application.ItemBrowse{})
	vaultBar := newFilterBar("/vaults/vault-1", compendiumViewTiles, compendiumViews, application.CompendiumFilters{Query: "torch"}, application.ItemBrowse{})

	compendiumHTML := renderPartial(t, "item_filter_bar", compendiumBar)
	vaultHTML := renderPartial(t, "item_filter_bar", vaultBar)

	if !strings.Contains(compendiumHTML, `action="/compendium"`) {
		t.Fatalf("expected the compendium filter bar to submit to /compendium, got:\n%s", compendiumHTML)
	}
	if !strings.Contains(vaultHTML, `action="/vaults/vault-1"`) {
		t.Fatalf("expected the vault filter bar to submit to /vaults/vault-1, got:\n%s", vaultHTML)
	}
	// Both must repopulate the same active filter value: it's the same partial,
	// only the action/reset URLs and the underlying data differ.
	for _, html := range []string{compendiumHTML, vaultHTML} {
		if !strings.Contains(html, `name="q" value="torch"`) {
			t.Fatalf("expected the active query filter to be repopulated, got:\n%s", html)
		}
	}
}

func TestItemCardsPartialRendersTheSameMarkupForBothPages(t *testing.T) {
	t.Parallel()

	entries := []application.ItemEntry{{
		Item:          domain.Item{ID: "torch-1", Name: "Torch", Category: "equipment", Rarity: domain.RarityMundane},
		LocationLabel: "Vault root",
		UnitWeightLB:  "1",
		UnitValueText: "1 cp",
	}}

	html := renderPartial(t, "item_cards", entries)

	if !strings.Contains(html, "data-card-grid") || !strings.Contains(html, "data-item-card") {
		t.Fatalf("expected the shared card grid markup, got:\n%s", html)
	}
	if !strings.Contains(html, ">Torch<") {
		t.Fatalf("expected the card to show the item name, got:\n%s", html)
	}
}

func TestItemTableShowsLocationLabelForRootEntries(t *testing.T) {
	t.Parallel()

	entries := []application.ItemEntry{{
		Item:          domain.Item{ID: "torch-1", Name: "Torch", Category: "equipment"},
		LocationLabel: "Vault root",
		UnitWeightLB:  "1",
		UnitValueText: "1 cp",
	}}

	html := renderPartial(t, "item_table", entries)

	if !strings.Contains(html, "Vault root") {
		t.Fatalf("expected the table to show the entry's root location label, got:\n%s", html)
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

func TestItemFormWeaponPropertiesRendersCheckboxes(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	err = server.tmpl.ExecuteTemplate(&rendered, "item_form_fields", TemplateData{
		Categories: domain.Categories(),
		Rarities:   domain.Rarities(),
		Item:       domain.Item{Category: "weapon"},
	})
	if err != nil {
		t.Fatalf("render item form fields: %v", err)
	}

	html := rendered.String()

	// All ten canonical weapon properties must render as checkboxes.
	for _, prop := range domain.WeaponProperties() {
		checkbox := `<input type="checkbox" name="weapon_properties" value="` + prop.Key + `"`
		if !strings.Contains(html, checkbox) {
			t.Errorf("expected checkbox for property %q, got:\n%s", prop.Key, html)
		}
		if !strings.Contains(html, prop.Label) {
			t.Errorf("expected label text %q for property %q", prop.Label, prop.Key)
		}
		if !strings.Contains(html, prop.Description) {
			t.Errorf("expected description text for property %q", prop.Key)
		}
	}

	// The old free-text input must be gone.
	if strings.Contains(html, `<input type="text" name="weapon_properties"`) {
		t.Fatalf("expected no free-text weapon_properties input, but found one")
	}
}

func TestItemFormWeaponPropertiesPreChecksStoredValue(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	item := domain.Item{Category: "weapon"}
	item.Details.Weapon = &domain.WeaponDetails{
		WeaponClass: "martial-melee",
		Properties:  []string{"versatile"},
	}

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
	// "versatile" must be checked.
	if !strings.Contains(html, `value="versatile" checked`) {
		t.Fatalf("expected versatile checkbox to be pre-checked, got:\n%s", html)
	}
	// Other canonical properties must not be checked.
	if strings.Contains(html, `value="thrown" checked`) {
		t.Fatalf("expected thrown checkbox to be unchecked")
	}
}

func TestItemFormWeaponPropertiesRendersLegacyUnknownProperty(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	item := domain.Item{Category: "weapon"}
	item.Details.Weapon = &domain.WeaponDetails{Properties: []string{"glowing"}}

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
	// An unknown property must appear as an extra checked checkbox.
	if !strings.Contains(html, `value="glowing" checked`) {
		t.Fatalf("expected legacy glowing checkbox to be pre-checked, got:\n%s", html)
	}
	// Its label must be flagged legacy so it renders visually distinct.
	if !strings.Contains(html, "data-legacy") {
		t.Fatalf("expected a data-legacy marker on the legacy property's checkbox field, got:\n%s", html)
	}
}

func TestItemFormWeaponPropertiesThrownDetectionScript(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	err = server.tmpl.ExecuteTemplate(&rendered, "item_form_fields", TemplateData{
		Categories: domain.Categories(),
		Rarities:   domain.Rarities(),
		Item:       domain.Item{Category: "weapon"},
	})
	if err != nil {
		t.Fatalf("render item form fields: %v", err)
	}

	html := rendered.String()

	// Thrown detection must use checkbox semantics, not text-input comma-splitting.
	for _, fragment := range []string{
		`input[name="weapon_properties"]`,
		`box.checked`,
	} {
		if !strings.Contains(html, fragment) {
			t.Errorf("expected thrown-detection script to contain %q", fragment)
		}
	}

	// The existing weaponClassSelect listener and range-fields container must remain.
	for _, fragment := range []string{
		`data-weapon-range-fields`,
		`weaponClassSelect.addEventListener`,
	} {
		if !strings.Contains(html, fragment) {
			t.Errorf("expected form script to still contain %q", fragment)
		}
	}

	// The old text-input event listener must be gone.
	if strings.Contains(html, `propertiesInput`) {
		t.Fatalf("expected old propertiesInput variable to be removed from script")
	}
}

func TestItemFormVersatileDetectionScript(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	err = server.tmpl.ExecuteTemplate(&rendered, "item_form_fields", TemplateData{
		Categories: domain.Categories(),
		Rarities:   domain.Rarities(),
		Item:       domain.Item{Category: "weapon"},
	})
	if err != nil {
		t.Fatalf("render item form fields: %v", err)
	}

	html := rendered.String()

	// The versatile field wrapper and input must appear in the rendered form.
	for _, fragment := range []string{
		`data-weapon-versatile-fields`,
		`weapon_versatile_damage_dice`,
	} {
		if !strings.Contains(html, fragment) {
			t.Errorf("expected versatile form fields to contain %q", fragment)
		}
	}

	// The versatile show/hide logic must be present and keyed on the versatile checkbox.
	for _, fragment := range []string{
		`versatileFields`,
		`"versatile"`,
	} {
		if !strings.Contains(html, fragment) {
			t.Errorf("expected versatile detection script to contain %q", fragment)
		}
	}
}

func TestCompendiumRendersTilesByDefault(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{}))

	if !strings.Contains(html, `data-card-grid`) {
		t.Fatalf("expected a card grid in the tile view, got:\n%s", html)
	}
	if !strings.Contains(html, `data-item-card`) {
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

	name := strings.Index(html, `data-card-title`)
	meta := strings.Index(html, `data-card-meta`)
	stats := strings.Index(html, `data-card-stats`)
	// Anchored on the element, not the text: the description also appears in the card tooltip.
	description := strings.Index(html, `data-card-desc`)

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
	card := strings.Index(html, `<article data-item-card`)
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
	if strings.Contains(html, `data-card-desc`) {
		t.Fatalf("expected no description paragraph when the item has no description, got:\n%s", html)
	}
}

func TestCompendiumListViewRendersTable(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, compendiumTemplateData(compendiumViewList, application.CompendiumFilters{}))

	if !strings.Contains(html, "<table") {
		t.Fatalf("expected a table in the list view, got:\n%s", html)
	}
	if strings.Contains(html, `data-card-grid`) {
		t.Fatalf("expected no card grid in the list view, got:\n%s", html)
	}
	if !strings.Contains(html, "<th>Location</th>") {
		t.Fatalf("expected the list view to show the location column, got:\n%s", html)
	}
	if !strings.Contains(html, "Oak Chest") {
		t.Fatalf("expected the list view to show the nested item's container path, got:\n%s", html)
	}
}

func TestCompendiumCardClampsDescription(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{}))

	if !strings.Contains(html, "line-clamp-3") {
		t.Fatalf("expected the card description to be clamped via line-clamp-3, got:\n%s", html)
	}
}

func TestCompendiumCardStretchesTitleLink(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{}))

	card := strings.Index(html, `<article data-item-card`)
	if card < 0 {
		t.Fatalf("expected an item card article, got:\n%s", html)
	}
	openingTag := html[card:]
	if end := strings.Index(openingTag, ">"); end >= 0 {
		openingTag = openingTag[:end]
	}
	if !strings.Contains(openingTag, "relative") {
		t.Fatalf("expected the card to establish a positioning context via `relative`, got:\n%s", openingTag)
	}
	if !strings.Contains(html, "after:absolute") || !strings.Contains(html, "after:inset-0") {
		t.Fatalf("expected the card title link to be stretched over the tile via after:absolute after:inset-0, got:\n%s", html)
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

// weaponCompendiumTemplateData builds a one-weapon-entry compendium for chip tests.
// Kept separate from compendiumTemplateData so the card-count assertion there stays valid.
func weaponCompendiumTemplateData(view string) TemplateData {
	longbow := domain.Item{
		ID:                 "longbow-1",
		Name:               "Longbow",
		Category:           "weapon",
		Rarity:             domain.RarityMundane,
		Description:        "A tall bow made of flexible wood.",
		WeightHundredthsLB: 200,
		BaseValueCP:        5000,
		Quantity:           1,
	}
	longbow.Details.Weapon = &domain.WeaponDetails{
		WeaponClass: "martial-ranged",
		Properties:  []string{"thrown", "silvered"}, // one canonical, one legacy
	}

	browse := application.CompendiumBrowse{
		Entries: []application.CompendiumEntry{{
			Item:           longbow,
			ContainerPath:  "",
			IsNested:       false,
			LocationLabel:  "Compendium root",
			UnitWeightLB:   "2",
			UnitValueText:  "50 gp",
			TotalWeightLB:  "2",
			TotalValueText: "50 gp",
		}},
		Facets:     domain.CompendiumFacets{WeaponProperties: []string{"silvered"}},
		TotalCount: 1,
		MatchCount: 1,
	}

	return TemplateData{
		Title:             "Compendium",
		Categories:        domain.Categories(),
		Rarities:          domain.Rarities(),
		CompendiumView:    view,
		CompendiumFilters: application.CompendiumFilters{},
		Compendium:        browse,
		FilterBar:         newFilterBar("/compendium", view, compendiumViews, application.CompendiumFilters{}, browse),
	}
}

func renderItemDetail(t *testing.T, data TemplateData) string {
	t.Helper()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	if err := server.tmpl.ExecuteTemplate(&rendered, "item_detail", data); err != nil {
		t.Fatalf("render item_detail: %v", err)
	}
	return rendered.String()
}

func TestWeaponPropertyChipsRenderOnCompendiumCard(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, weaponCompendiumTemplateData(compendiumViewTiles))

	// Canonical "thrown" chip must have a label, tooltip, and aria wiring.
	if !strings.Contains(html, `>Thrown<`) {
		t.Fatalf("expected Thrown chip label, got:\n%s", html)
	}
	if !strings.Contains(html, `aria-describedby="wp-longbow-1-thrown"`) {
		t.Fatalf("expected aria-describedby for thrown chip, got:\n%s", html)
	}
	if !strings.Contains(html, `id="wp-longbow-1-thrown"`) {
		t.Fatalf("expected tooltip element id for thrown chip, got:\n%s", html)
	}
	// The title attribute must carry the thrown description (attribute context, no escaping of plain text).
	thrownDesc := domain.WeaponProperties()[7].Description // thrown is index 7
	if !strings.Contains(html, `title="`+thrownDesc+`"`) {
		t.Fatalf("expected title attribute with thrown description, got:\n%s", html)
	}

	// Unknown "silvered" chip must have the legacy class and no aria-describedby.
	if !strings.Contains(html, `prop-chip--unknown`) {
		t.Fatalf("expected prop-chip--unknown class for silvered chip, got:\n%s", html)
	}
	if strings.Contains(html, `aria-describedby="wp-longbow-1-silvered"`) {
		t.Fatalf("expected no aria-describedby for unknown silvered chip, got:\n%s", html)
	}

	// The card must still have exactly one item link.
	card := strings.Index(html, `<article data-item-card`)
	if card < 0 {
		t.Fatalf("expected an item card article, got:\n%s", html)
	}
	if count := strings.Count(html[card:], `<a href="/items/`); count != 1 {
		t.Fatalf("expected exactly one item link per card, got %d:\n%s", count, html[card:])
	}
}

func TestWeaponPropertyChipsRenderOnCompendiumListColumn(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, weaponCompendiumTemplateData(compendiumViewList))

	if !strings.Contains(html, `<th>Properties</th>`) {
		t.Fatalf("expected Properties column header in list view, got:\n%s", html)
	}
	// Thrown chip must appear in the table row.
	if !strings.Contains(html, `>Thrown<`) {
		t.Fatalf("expected Thrown chip in list view row, got:\n%s", html)
	}
}

func TestWeaponPropertyChipsRenderOnItemDetail(t *testing.T) {
	t.Parallel()

	longbow := domain.Item{
		ID:       "longbow-detail-1",
		Name:     "Longbow",
		Category: "weapon",
	}
	longbow.Details.Weapon = &domain.WeaponDetails{
		Properties: []string{"finesse"},
	}

	data := TemplateData{
		Categories: domain.Categories(),
		Rarities:   domain.Rarities(),
		AllVaults:  []domain.Vault{},
		ItemDetail: application.ItemDetail{Item: longbow},
	}

	html := renderItemDetail(t, data)

	if !strings.Contains(html, "Weapon properties") {
		t.Fatalf("expected Weapon properties heading on item detail page, got:\n%s", html)
	}
	if !strings.Contains(html, `>Finesse<`) {
		t.Fatalf("expected Finesse chip on item detail page, got:\n%s", html)
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

func TestCompendiumWeaponPropertyDropdownContainsAllCanonicalAndLegacyOptions(t *testing.T) {
	t.Parallel()

	// The dropdown must always show all ten canonical keys, even with an empty compendium.
	html := renderCompendium(t, weaponCompendiumTemplateData(compendiumViewTiles))

	for _, prop := range domain.WeaponProperties() {
		option := `<option value="` + prop.Key + `"`
		if !strings.Contains(html, option) {
			t.Errorf("expected canonical option %q in weapon property dropdown, got:\n%s", prop.Key, html)
		}
	}

	// The legacy facet value "silvered" (from the fixture) must also appear in the dropdown.
	if !strings.Contains(html, `<option value="silvered"`) {
		t.Fatalf("expected legacy silvered option in weapon property dropdown, got:\n%s", html)
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
