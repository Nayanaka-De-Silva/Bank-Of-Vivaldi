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

// detailsBodyRange returns the byte range of a data-panel="key" details
// element's body, i.e. everything between its own opening tag's ">" and its
// own matching "</details>". Both helpers below use this to prove an element
// is (or isn't) nested inside a specific panel, not just that both substrings
// appear somewhere in the page.
func detailsBodyRange(t *testing.T, html, panelKey string) (start, end int) {
	t.Helper()

	tag := panelTag(t, html, panelKey)
	tagIndex := strings.Index(html, tag)
	if tagIndex < 0 {
		t.Fatalf("expected to relocate the %q panel tag, got:\n%s", panelKey, html)
	}
	start = tagIndex + len(tag)
	closeOffset := strings.Index(html[start:], "</details>")
	if closeOffset < 0 {
		t.Fatalf("expected the %q panel to close, got:\n%s", panelKey, html)
	}
	return start, start + closeOffset
}

func TestFilterActionsSitInsideTheFilterPanelTheyControl(t *testing.T) {
	t.Parallel()

	// Apply/Reset act on the filter fields, so they belong inside that same
	// panel -- at the bottom, since the fields they submit sit above them
	// (user follow-up on issue #30).
	compendiumHTML := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{}))
	vaultHTML := renderVaultDetail(t, vaultTemplateDataWithView(vaultViewTiles, application.CompendiumFilters{}))

	panelKeys := map[string]string{"compendium": "compendium-filters", "vault": "vault-filters"}
	for name, html := range map[string]string{"compendium": compendiumHTML, "vault": vaultHTML} {
		start, end := detailsBodyRange(t, html, panelKeys[name])
		actionsIndex := strings.Index(html[start:end], "data-filter-actions")
		if actionsIndex < 0 {
			t.Fatalf("%s: expected the actions row inside the filter panel, got:\n%s", name, html[start:end])
		}
		// "At the bottom": nothing meaningful should follow the actions row
		// before the panel closes -- specifically, the field grid/category
		// groups must precede it, not the other way round.
		if !strings.Contains(html[start:start+actionsIndex], `name="q"`) {
			t.Fatalf("%s: expected the search field to precede the actions row, got:\n%s", name, html[start:end])
		}
	}
}

func TestViewToggleSitsAtTopOfTheResultsPanelItControls(t *testing.T) {
	t.Parallel()

	// The tiles/list/tree switch controls the results section's display, not
	// the filter criteria, so it belongs at the top of that panel, ahead of
	// the results it switches between (user follow-up on issue #30).
	compendiumHTML := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{}))
	vaultHTML := renderVaultDetail(t, vaultTemplateDataWithView(vaultViewTiles, application.CompendiumFilters{}))

	cases := []struct {
		name, html, panelKey, resultMarker string
	}{
		{"compendium", compendiumHTML, "compendium-results", ">Scale Mail<"},
		{"vault", vaultHTML, "vault-contents", ">Longsword<"},
	}
	for _, testCase := range cases {
		start, end := detailsBodyRange(t, testCase.html, testCase.panelKey)
		body := testCase.html[start:end]

		toggleIndex := strings.Index(body, `aria-label="Result layout"`)
		if toggleIndex < 0 {
			t.Fatalf("%s: expected the view toggle inside the results panel, got:\n%s", testCase.name, body)
		}
		resultIndex := strings.Index(body, testCase.resultMarker)
		if resultIndex < 0 {
			t.Fatalf("%s: expected a result entry, got:\n%s", testCase.name, body)
		}
		if toggleIndex > resultIndex {
			t.Fatalf("%s: expected the view toggle to sit above the results, got:\n%s", testCase.name, body)
		}
	}
}

func TestViewToggleSubmitsTheFilterFormFromOutsideIt(t *testing.T) {
	t.Parallel()

	// The toggle buttons render inside the results panel, physically outside
	// the <form> the filter fields live in (see item_filter_bar) -- they must
	// reference it by id via the `form` attribute or the current filters
	// wouldn't ride along with the view switch.
	html := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{Query: "mail"}))

	if !strings.Contains(html, `id="compendium-filters-form"`) {
		t.Fatalf("expected the filter form to carry a stable id, got:\n%s", html)
	}
	if !strings.Contains(html, `form="compendium-filters-form"`) {
		t.Fatalf("expected the view toggle buttons to target the filter form by id, got:\n%s", html)
	}
}

func TestCompendiumFilterPanelIgnoresSortOnlySelection(t *testing.T) {
	t.Parallel()

	// A non-default sort narrows nothing, so it must not force the filter
	// panel open or lock it against the restore script (code review, issue #30).
	html := renderCompendium(t, compendiumTemplateData(compendiumViewTiles, application.CompendiumFilters{SortBy: "value"}))

	tag := panelTag(t, html, "compendium-filters")
	if strings.Contains(tag, " open") || strings.Contains(tag, "data-panel-lock") {
		t.Fatalf("expected a sort-only selection to leave the filter panel closed and unlocked, got:\n%s", tag)
	}
	if !strings.Contains(html, "No filters applied") {
		t.Fatalf("expected the collapsed filter row to say no filters are applied, got:\n%s", html)
	}
}

func TestVaultTreeViewDoesNotClaimAFilteredMatchCount(t *testing.T) {
	t.Parallel()

	// The tree ignores filters entirely, so its own panel header must not read
	// like the tiles/list "Showing X of Y" count (code review, issue #30).
	data := vaultTemplateDataWithView(vaultViewTree, application.CompendiumFilters{})
	data.FilterBar.TotalCount = 40
	data.FilterBar.MatchCount = 2

	html := renderVaultDetail(t, data)
	if strings.Contains(html, "Showing 2 of 40") {
		t.Fatalf("expected the tree view not to report a filtered match count, got:\n%s", html)
	}
	if !strings.Contains(html, "40 items, unfiltered") {
		t.Fatalf("expected the tree view header to report the unfiltered total, got:\n%s", html)
	}
}

func TestDisclosureCaretRotatesOnlyForItsOwnDetails(t *testing.T) {
	t.Parallel()

	// Both the shared item tree caret and the page panel caret must render
	// through the disclosure-caret CSS hook, not a Tailwind group-open:
	// utility -- the utility's descendant selector leaks an outer open
	// ancestor's state onto an unrelated closed child (code review, issue #30).
	html := renderVaultDetail(t, vaultTemplateDataWithView(vaultViewTiles, application.CompendiumFilters{}))

	if !strings.Contains(html, `class="disclosure-caret`) {
		t.Fatalf("expected disclosure carets to carry the disclosure-caret CSS hook, got:\n%s", html)
	}
	if strings.Contains(html, "group-open:rotate-90") || strings.Contains(html, "group-open/panel:rotate-90") {
		t.Fatalf("expected no group-open rotate utility on a disclosure caret, got:\n%s", html)
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
