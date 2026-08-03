package httpui

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bank-of-vivaldi/internal/application"
	"bank-of-vivaldi/internal/domain"
)

func TestVaultRouteActionParsesIDAndAction(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path       string
		wantID     string
		wantAction string
	}{
		{"/vaults/x", "x", ""},
		{"/vaults/x/", "x", ""},
		{"/vaults/x/edit", "x", "edit"},
		{"/vaults/x/edit/", "x", "edit"},
		{"/vaults/x/purse", "x", "purse"},
		{"/vaults/", "", ""},
		{"/vaults/x/bogus", "x", "bogus"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			id, action := vaultRouteAction(tc.path)
			if id != tc.wantID || action != tc.wantAction {
				t.Fatalf("vaultRouteAction(%q) = (%q, %q), want (%q, %q)", tc.path, id, action, tc.wantID, tc.wantAction)
			}
		})
	}
}

// vaultTemplateData builds a one-vault VaultDetail fixture for vault_detail /
// vault_edit render tests, modelled on compendiumTemplateData.
func vaultTemplateData() TemplateData {
	return vaultTemplateDataWithView(vaultViewTiles, application.CompendiumFilters{})
}

// vaultTemplateDataWithView builds the same fixture, additionally wiring
// VaultBrowse/VaultView/FilterBar the way handleVaultDetail's GET branch does,
// for tests that exercise the filterable tiles/list/tree browser.
func vaultTemplateDataWithView(view string, filters application.CompendiumFilters) TemplateData {
	vault := domain.Vault{
		ID:              "vault-1",
		CharacterName:   "Aragorn",
		StrengthScore:   16,
		CarryModifierLB: 0,
		EncumbranceMode: domain.EncumbranceModeStandard,
		Notes:           "Ranger of the North",
		Purse:           domain.Purse{CP: 1, SP: 2, EP: 0, GP: 30, PP: 1},
	}

	sword := domain.Item{ID: "sword", Name: "Longsword", Category: "weapon", Rarity: domain.RarityMundane}

	detail := application.VaultDetail{
		Summary: application.VaultSummary{
			Vault:                      vault,
			TotalCarryWeightHundredths: 1000,
			MaxCarryWeightHundredths:   24000,
			EncumbranceState:           domain.EncumbranceStateNormal,
		},
		RootItems: []application.ItemNode{{Item: sword}},
	}

	browse := application.ItemBrowse{
		Entries: []application.ItemEntry{{
			Item:          sword,
			LocationLabel: "Vault root",
			UnitWeightLB:  "3",
			UnitValueText: "10 gp",
		}},
		TotalCount: 1,
		MatchCount: 1,
	}

	return TemplateData{
		Title:       "Aragorn",
		VaultDetail: detail,
		VaultBrowse: browse,
		VaultView:   view,
		FilterBar:   newFilterBar("/vaults/vault-1", view, vaultViews, filters, browse),
	}
}

func renderVaultDetail(t *testing.T, data TemplateData) string {
	t.Helper()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	if err := server.tmpl.ExecuteTemplate(&rendered, "vault_detail", data); err != nil {
		t.Fatalf("render vault_detail: %v", err)
	}
	return rendered.String()
}

func renderVaultEdit(t *testing.T, data TemplateData) string {
	t.Helper()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	if err := server.tmpl.ExecuteTemplate(&rendered, "vault_edit", data); err != nil {
		t.Fatalf("render vault_edit: %v", err)
	}
	return rendered.String()
}

func TestVaultDetailHidesEditFormsBehindAButton(t *testing.T) {
	t.Parallel()

	html := renderVaultDetail(t, vaultTemplateData())

	if !strings.Contains(html, `href="/vaults/vault-1/edit"`) {
		t.Fatalf("expected an edit-vault link to /vaults/vault-1/edit, got:\n%s", html)
	}
	if strings.Contains(html, `name="character_name"`) {
		t.Fatalf("expected no character_name field on the detail page, got:\n%s", html)
	}
	if strings.Contains(html, `action="/vaults/vault-1/purse"`) {
		t.Fatalf("expected no purse form on the detail page, got:\n%s", html)
	}
}

func TestVaultEditRendersBothFormsWithUnchangedPostTargets(t *testing.T) {
	t.Parallel()

	html := renderVaultEdit(t, vaultTemplateData())

	if !strings.Contains(html, `action="/vaults/vault-1"`) {
		t.Fatalf("expected the vault form to post to /vaults/vault-1, got:\n%s", html)
	}
	if !strings.Contains(html, `action="/vaults/vault-1/purse"`) {
		t.Fatalf("expected the purse form to post to /vaults/vault-1/purse, got:\n%s", html)
	}
	for _, name := range []string{
		"character_name", "strength_score", "carry_modifier_lb",
		"encumbrance_mode", "notes", "archived",
		"cp", "sp", "ep", "gp", "pp",
	} {
		if !strings.Contains(html, `name="`+name+`"`) {
			t.Fatalf("expected field %q to survive the split onto the edit page, got:\n%s", name, html)
		}
	}
	if !strings.Contains(html, `value="Aragorn"`) {
		t.Fatalf("expected the vault's current values to be pre-filled, got:\n%s", html)
	}
}

func TestVaultEditIsReachableFromTheDetailPage(t *testing.T) {
	t.Parallel()

	detailHTML := renderVaultDetail(t, vaultTemplateData())
	editHTML := renderVaultEdit(t, vaultTemplateData())

	if !strings.Contains(detailHTML, "/vaults/vault-1/edit") {
		t.Fatalf("expected the detail page to link to the edit page, got:\n%s", detailHTML)
	}
	// The edit page itself must link back to the detail page.
	if !strings.Contains(editHTML, `href="/vaults/vault-1"`) {
		t.Fatalf("expected the edit page to link back to the detail page, got:\n%s", editHTML)
	}
}

func TestNormalizeVaultView(t *testing.T) {
	t.Parallel()

	if got := normalizeVaultView(""); got != vaultViewTiles {
		t.Fatalf("default view = %q, want %q", got, vaultViewTiles)
	}
	if got := normalizeVaultView("list"); got != vaultViewList {
		t.Fatalf("view = %q, want %q", got, vaultViewList)
	}
	if got := normalizeVaultView("tree"); got != vaultViewTree {
		t.Fatalf("view = %q, want %q", got, vaultViewTree)
	}
	if got := normalizeVaultView("TREE"); got != vaultViewTree {
		t.Fatalf("uppercase view = %q, want %q (matching normalizeCompendiumView's case-insensitivity)", got, vaultViewTree)
	}
	if got := normalizeVaultView("nonsense"); got != vaultViewTiles {
		t.Fatalf("unknown view = %q, want the tiles default", got)
	}
}

func TestVaultFiltersReuseTheCompendiumQueryParams(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/vaults/vault-1?q=sword&category=weapon", nil)
	filters := parseCompendiumFilters(req)

	if filters.Query != "sword" || filters.Category != "weapon" {
		t.Fatalf("filters = %+v, want Query=sword Category=weapon", filters)
	}
}

func TestVaultDetailRendersTilesByDefault(t *testing.T) {
	t.Parallel()

	html := renderVaultDetail(t, vaultTemplateDataWithView(vaultViewTiles, application.CompendiumFilters{}))

	if !strings.Contains(html, "data-card-grid") || !strings.Contains(html, "data-item-card") {
		t.Fatalf("expected the vault browser to default to cards, got:\n%s", html)
	}
	if strings.Contains(html, "<table") {
		t.Fatalf("expected no table in the default tiles view, got:\n%s", html)
	}
	if !strings.Contains(html, ">Longsword<") {
		t.Fatalf("expected the sword entry to appear on a card, got:\n%s", html)
	}
}

func TestVaultDetailListViewRendersTable(t *testing.T) {
	t.Parallel()

	html := renderVaultDetail(t, vaultTemplateDataWithView(vaultViewList, application.CompendiumFilters{}))

	if !strings.Contains(html, "<table") {
		t.Fatalf("expected a table in the list view, got:\n%s", html)
	}
	if strings.Contains(html, "data-card-grid") {
		t.Fatalf("expected no card grid in the list view, got:\n%s", html)
	}
	if !strings.Contains(html, "Vault root") {
		t.Fatalf("expected the list view to show the vault-root location label, got:\n%s", html)
	}
}

func TestVaultDetailTreeViewRendersTheRecursiveItemNodes(t *testing.T) {
	t.Parallel()

	html := renderVaultDetail(t, vaultTemplateDataWithView(vaultViewTree, application.CompendiumFilters{}))

	if !strings.Contains(html, "item-tree") {
		t.Fatalf("expected the tree view to render the recursive item_nodes partial, got:\n%s", html)
	}
	if strings.Contains(html, "data-card-grid") || strings.Contains(html, "<table") {
		t.Fatalf("expected no cards or table in the tree view, got:\n%s", html)
	}
}

func TestVaultDetailTreeViewNotesThatFiltersDoNotApply(t *testing.T) {
	t.Parallel()

	html := renderVaultDetail(t, vaultTemplateDataWithView(vaultViewTree, application.CompendiumFilters{}))

	if !strings.Contains(html, "Filters apply to the tiles and list views") {
		t.Fatalf("expected a note that filters don't apply in tree view, got:\n%s", html)
	}
}

func TestVaultDetailViewToggleOffersTilesListAndTree(t *testing.T) {
	t.Parallel()

	html := renderVaultDetail(t, vaultTemplateDataWithView(vaultViewTiles, application.CompendiumFilters{}))

	for _, view := range []string{"tiles", "list", "tree"} {
		if !strings.Contains(html, `name="view" value="`+view+`"`) {
			t.Fatalf("expected a %s-view submit button, got:\n%s", view, html)
		}
	}
}

func TestVaultDetailFilterBarSubmitsToTheVaultURL(t *testing.T) {
	t.Parallel()

	html := renderVaultDetail(t, vaultTemplateDataWithView(vaultViewTiles, application.CompendiumFilters{}))

	if !strings.Contains(html, `action="/vaults/vault-1"`) {
		t.Fatalf("expected the filter bar to submit to the vault's own URL, got:\n%s", html)
	}
}

func TestVaultDetailShowsMatchCounts(t *testing.T) {
	t.Parallel()

	html := renderVaultDetail(t, vaultTemplateDataWithView(vaultViewTiles, application.CompendiumFilters{}))

	if !strings.Contains(html, "1 of 1") {
		t.Fatalf("expected the matched/total item counts to be shown, got:\n%s", html)
	}
}

func TestVaultDetailEmptyResultExplainsTheFilter(t *testing.T) {
	t.Parallel()

	data := vaultTemplateDataWithView(vaultViewTiles, application.CompendiumFilters{Category: "armor"})
	data.VaultBrowse.Entries = nil
	data.VaultBrowse.MatchCount = 0

	html := renderVaultDetail(t, data)
	if !strings.Contains(html, "No items match") {
		t.Fatalf("expected an empty-result message, got:\n%s", html)
	}
}

// TestVaultDetailTreeViewRendersContainerWithChildrenAsDisclosure asserts that a
// container ItemNode with non-empty Children renders a <details>/<summary>
// disclosure rather than a plain flat row, and that the child item's name
// appears in the output.
func TestVaultDetailTreeViewRendersContainerWithChildrenAsDisclosure(t *testing.T) {
	t.Parallel()

	backpack := domain.Item{
		ID:          "backpack",
		Name:        "Backpack",
		Category:    "container",
		Rarity:      domain.RarityMundane,
		IsContainer: true,
	}
	dagger := domain.Item{ID: "dagger", Name: "Dagger", Category: "weapon", Rarity: domain.RarityMundane}

	data := vaultTemplateDataWithView(vaultViewTree, application.CompendiumFilters{})
	data.VaultDetail.RootItems = []application.ItemNode{
		{
			Item: backpack,
			Children: []application.ItemNode{
				{Item: dagger},
			},
		},
	}

	html := renderVaultDetail(t, data)

	if !strings.Contains(html, "item-tree") {
		t.Fatalf("expected item-tree class in tree view, got:\n%s", html)
	}
	if !strings.Contains(html, "<details") {
		t.Fatalf("expected a <details> disclosure element for the container, got:\n%s", html)
	}
	if !strings.Contains(html, "<summary") {
		t.Fatalf("expected a <summary> element for the container header, got:\n%s", html)
	}
	if !strings.Contains(html, "Dagger") {
		t.Fatalf("expected the child item name to appear in the rendered output, got:\n%s", html)
	}
	if strings.Contains(html, "data-card-grid") || strings.Contains(html, "<table") {
		t.Fatalf("expected no card grid or table in tree view, got:\n%s", html)
	}
}

// TestVaultDetailTreeViewNestedContainerStartsCollapsed asserts the depth rule:
// a top-level container renders <details open> while a container nested inside
// another container renders <details> WITHOUT the open attribute (collapsed by
// default).
func TestVaultDetailTreeViewNestedContainerStartsCollapsed(t *testing.T) {
	t.Parallel()

	chest := domain.Item{ID: "chest", Name: "Chest", Category: "container", Rarity: domain.RarityMundane, IsContainer: true}
	innerBag := domain.Item{ID: "bag", Name: "Bag of Holding", Category: "container", Rarity: domain.RarityRare, IsContainer: true}
	dagger := domain.Item{ID: "dagger", Name: "Dagger", Category: "weapon", Rarity: domain.RarityMundane}

	data := vaultTemplateDataWithView(vaultViewTree, application.CompendiumFilters{})
	data.VaultDetail.RootItems = []application.ItemNode{
		{
			Item: chest,
			Children: []application.ItemNode{
				{
					Item: innerBag,
					Children: []application.ItemNode{
						{Item: dagger},
					},
				},
			},
		},
	}

	html := renderVaultDetail(t, data)

	// Top-level container must render with the open attribute.
	if !strings.Contains(html, `<details class="group" open>`) {
		t.Fatalf("expected top-level container to render with open attribute, got:\n%s", html)
	}
	// Nested container must render WITHOUT the open attribute (collapsed).
	if !strings.Contains(html, `<details class="group">`) {
		t.Fatalf("expected nested container to render without open attribute, got:\n%s", html)
	}
	// The leaf item must still appear in the output (nested inside collapsed disclosure).
	if !strings.Contains(html, "Dagger") {
		t.Fatalf("expected leaf item name to appear in the output, got:\n%s", html)
	}
}
