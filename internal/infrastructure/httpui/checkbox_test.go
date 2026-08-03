package httpui

import (
	"bytes"
	"strings"
	"testing"

	"bank-of-vivaldi/internal/domain"
)

// renderCheckboxField renders the checkbox_field partial directly against a
// checkboxField view model.
func renderCheckboxField(t *testing.T, field checkboxField) string {
	t.Helper()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	if err := server.tmpl.ExecuteTemplate(&rendered, "checkbox_field", field); err != nil {
		t.Fatalf("render checkbox_field: %v", err)
	}
	return rendered.String()
}

func TestCheckboxFieldRendersLabelColonThenInput(t *testing.T) {
	t.Parallel()

	html := renderCheckboxField(t, checkbox("Is container", "is_container", false))

	label := strings.Index(html, ">Is container:<")
	input := strings.Index(html, `<input type="checkbox" name="is_container"`)
	if label < 0 || input < 0 {
		t.Fatalf("expected the label text followed by the checkbox input, got:\n%s", html)
	}
	if label > input {
		t.Fatalf("expected the label text to precede the input (Field name: []), got:\n%s", html)
	}
	// The old global `input { width: 100% }` rule (the root cause of the
	// original bug) must not be able to stretch this checkbox: no w-full.
	if strings.Contains(html, "w-full") {
		t.Fatalf("expected the checkbox to not carry w-full, got:\n%s", html)
	}
}

func TestCheckboxFieldKeepsValueAttributeForOptionGroups(t *testing.T) {
	t.Parallel()

	choice := domain.WeaponPropertyChoice{
		WeaponProperty: domain.WeaponProperty{Key: "versatile", Label: "Versatile", Description: "Can be used one- or two-handed."},
		Selected:       true,
		Known:          true,
	}
	html := renderCheckboxField(t, weaponPropertyCheckbox(choice))

	if !strings.Contains(html, `value="versatile" checked`) {
		t.Fatalf("expected the value attribute and checked state to survive, got:\n%s", html)
	}
	if !strings.Contains(html, "Can be used one- or two-handed.") {
		t.Fatalf("expected the property description as a hint, got:\n%s", html)
	}
}

func TestItemFormCheckboxesUseTheInlineFieldPartial(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	err = server.tmpl.ExecuteTemplate(&rendered, "item_form_fields", TemplateData{
		Categories: domain.Categories(),
		Rarities:   domain.Rarities(),
		Item:       domain.Item{Category: "armor"},
	})
	if err != nil {
		t.Fatalf("render item form fields: %v", err)
	}

	html := rendered.String()
	for _, name := range []string{"is_container", "is_stackable", "is_equipped", "is_magical", "requires_attunement", "armor_stealth_disadvantage"} {
		if !strings.Contains(html, `<input type="checkbox" name="`+name+`"`) {
			t.Fatalf("expected a checkbox field for %q, got:\n%s", name, html)
		}
	}
}

func TestBulkCheckboxesUseTheInlineFieldPartial(t *testing.T) {
	t.Parallel()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	err = server.tmpl.ExecuteTemplate(&rendered, "bulk", TemplateData{
		Categories:           domain.Categories(),
		Rarities:             domain.Rarities(),
		SelectedLocationKind: string(domain.LocationKindCompendiumRoot),
	})
	if err != nil {
		t.Fatalf("render bulk: %v", err)
	}

	html := rendered.String()
	for _, name := range []string{"is_stackable", "is_magical", "requires_attunement", "is_equipped"} {
		if !strings.Contains(html, `<input type="checkbox" name="`+name+`"`) {
			t.Fatalf("expected a checkbox field for %q, got:\n%s", name, html)
		}
	}
}

func TestVaultEditArchivedCheckboxUsesTheInlineFieldPartial(t *testing.T) {
	t.Parallel()

	html := renderVaultEdit(t, vaultTemplateData())

	if !strings.Contains(html, `<input type="checkbox" name="archived"`) {
		t.Fatalf("expected the archived checkbox to render via the inline field partial, got:\n%s", html)
	}
	if !strings.Contains(html, ">Archived:<") {
		t.Fatalf("expected the archived field label to read \"Archived:\", got:\n%s", html)
	}
}
