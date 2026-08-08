package httpui

import (
	"bytes"
	"strings"
	"testing"

	"bank-of-vivaldi/internal/application"
	"bank-of-vivaldi/internal/domain"
)

// panelTag returns the opening <details> tag carrying data-panel="key", so a test
// can assert on the panel's own attributes without matching the whole page.
func panelTag(t *testing.T, html, key string) string {
	t.Helper()

	marker := `data-panel="` + key + `"`
	markerIndex := strings.Index(html, marker)
	if markerIndex < 0 {
		t.Fatalf("expected a panel keyed %q, got:\n%s", key, html)
	}
	start := strings.LastIndex(html[:markerIndex], "<details")
	if start < 0 {
		t.Fatalf("expected panel %q to be a <details> disclosure, got:\n%s", key, html)
	}
	end := strings.Index(html[markerIndex:], ">")
	if end < 0 {
		t.Fatalf("expected panel %q to have a closed opening tag, got:\n%s", key, html)
	}
	return html[start : markerIndex+end+1]
}

// assertPanelOpen checks the server-rendered default open state of a panel.
func assertPanelOpen(t *testing.T, html, key string, wantOpen bool) {
	t.Helper()

	tag := panelTag(t, html, key)
	gotOpen := strings.Contains(tag, " open")
	if gotOpen != wantOpen {
		t.Fatalf("panel %q open = %v, want %v; tag was:\n%s", key, gotOpen, wantOpen, tag)
	}
	// The caret rotation and hover styling both hang off the group modifier.
	if !strings.Contains(tag, "group") {
		t.Fatalf("expected panel %q to carry the `group` class for its caret, got:\n%s", key, tag)
	}
}

func TestCompendiumSectionsRenderAsCollapsiblePanels(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{}))

	assertPanelOpen(t, html, "compendium-header", true)
	assertPanelOpen(t, html, "compendium-results", true)
	// Issue #30: the filter section is the one panel that starts minimized.
	assertPanelOpen(t, html, "compendium-filters", false)

	if !strings.Contains(html, "No filters applied") {
		t.Fatalf("expected the collapsed filter row to say no filters are applied, got:\n%s", html)
	}
}

func TestCompendiumFilterPanelOpensWhenFiltersAreActive(t *testing.T) {
	t.Parallel()

	filters := application.CompendiumFilters{Query: "mail", Rarity: "rare"}
	html := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, filters))

	tag := panelTag(t, html, "compendium-filters")
	if !strings.Contains(tag, " open") {
		t.Fatalf("expected an active filter set to open its panel, got:\n%s", tag)
	}
	// The lock stops the restore script from re-collapsing a panel the server
	// deliberately opened because filters are narrowing the results.
	if !strings.Contains(tag, "data-panel-lock") {
		t.Fatalf("expected the auto-opened filter panel to be locked open, got:\n%s", tag)
	}
	if !strings.Contains(html, "2 filters active") {
		t.Fatalf("expected the filter row to report the active filter count, got:\n%s", html)
	}
}

func TestVaultDetailSectionsRenderAsCollapsiblePanels(t *testing.T) {
	t.Parallel()

	html := renderVaultDetail(t, vaultTemplateDataWithView(vaultViewTiles, application.CompendiumFilters{}))

	assertPanelOpen(t, html, "vault-summary", true)
	assertPanelOpen(t, html, "vault-contents", true)
	assertPanelOpen(t, html, "vault-filters", false)
}

func TestItemDetailKeepsDialogsOutsideTheCollapsiblePanel(t *testing.T) {
	t.Parallel()

	data := itemTemplateData(domain.Item{ID: "item-1", Name: "Rope"})

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	var rendered bytes.Buffer
	if err := server.tmpl.ExecuteTemplate(&rendered, "item_detail", data); err != nil {
		t.Fatalf("render item_detail: %v", err)
	}
	html := rendered.String()

	assertPanelOpen(t, html, "item-summary", true)
	assertPanelOpen(t, html, "item-actions", true)

	// showModal() cannot paint a dialog nested in a closed <details> (display:none),
	// so the dialogs must sit after the actions panel closes.
	actionsIndex := strings.Index(html, `data-panel="item-actions"`)
	panelEnd := strings.Index(html[actionsIndex:], "</details>")
	dialogIndex := strings.Index(html, `<dialog id="move-dialog"`)
	if actionsIndex < 0 || panelEnd < 0 || dialogIndex < 0 {
		t.Fatalf("expected an actions panel and a move dialog, got:\n%s", html)
	}
	if dialogIndex < actionsIndex+panelEnd {
		t.Fatalf("expected the move dialog to sit outside the collapsible actions panel, got:\n%s", html)
	}
}

func TestFilterPanelHeaderReportsActiveFilters(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		filters   application.CompendiumFilters
		wantMeta  string
		wantBadge string
	}{
		{"none", application.CompendiumFilters{}, "No filters applied", ""},
		{"one", application.CompendiumFilters{Query: "rope"}, "", "1 filter active"},
		{"many", application.CompendiumFilters{Query: "rope", Rarity: "rare"}, "", "2 filters active"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			header := filterPanel("h2", "Filters", testCase.filters)
			if header.Meta != testCase.wantMeta {
				t.Fatalf("Meta = %q, want %q", header.Meta, testCase.wantMeta)
			}
			if header.Badge != testCase.wantBadge {
				t.Fatalf("Badge = %q, want %q", header.Badge, testCase.wantBadge)
			}
		})
	}
}

func TestPanelStatePersistsThroughLocalStorage(t *testing.T) {
	t.Parallel()

	html := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{}))

	if !strings.Contains(html, `"bov:panels"`) {
		t.Fatalf("expected a localStorage-backed panel state script, got:\n%s", html)
	}
	// data-panel-lock reaches the script as dataset.panelLock.
	if !strings.Contains(html, `"panelLock" in panel.dataset`) {
		t.Fatalf("expected the restore script to respect locked panels, got:\n%s", html)
	}
}
