package httpui

import (
	"bytes"
	"strings"
	"testing"

	"bank-of-vivaldi/internal/application"
	"bank-of-vivaldi/internal/domain"
)

// itemTemplateData builds a one-item ItemDetail fixture for item_detail /
// item_edit render tests, modelled on vaultTemplateData.
func itemTemplateData(item domain.Item) TemplateData {
	return TemplateData{
		Title:                     item.Name,
		ItemDetail:                application.ItemDetail{Item: item, LocationLabel: "Compendium root"},
		Item:                      item,
		Categories:                domain.Categories(),
		Rarities:                  domain.Rarities(),
		SelectedLocationKind:      string(item.Location.Kind),
		SelectedVaultID:           item.Location.OwnerVaultID,
		SelectedParentContainerID: item.Location.ParentContainerItemID,
	}
}

func renderItemEdit(t *testing.T, data TemplateData) string {
	t.Helper()

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	var rendered bytes.Buffer
	if err := server.tmpl.ExecuteTemplate(&rendered, "item_edit", data); err != nil {
		t.Fatalf("render item_edit: %v", err)
	}
	return rendered.String()
}

func TestItemDetailHidesEditFormBehindAButton(t *testing.T) {
	t.Parallel()

	html := renderItemDetailFromItem(t, domain.Item{ID: "item-1", Name: "Rope"})

	if !strings.Contains(html, `href="/items/item-1/edit"`) {
		t.Fatalf("expected an edit-item link to /items/item-1/edit, got:\n%s", html)
	}
	if strings.Contains(html, `name="category"`) {
		t.Fatalf("expected no category field (part of the edit form) on the detail page, got:\n%s", html)
	}
	if strings.Contains(html, `>Save item<`) {
		t.Fatalf("expected no Save item button on the detail page, got:\n%s", html)
	}
}

func TestItemEditRendersFormWithUnchangedPostTarget(t *testing.T) {
	t.Parallel()

	item := domain.Item{ID: "item-1", Name: "Rope", Category: "equipment", Rarity: domain.RarityMundane}
	html := renderItemEdit(t, itemTemplateData(item))

	if !strings.Contains(html, `action="/items/item-1"`) {
		t.Fatalf("expected the edit form to post to /items/item-1 (unchanged), got:\n%s", html)
	}
	if !strings.Contains(html, `name="category"`) || !strings.Contains(html, `name="rarity"`) {
		t.Fatalf("expected the edit form to carry the item fields, got:\n%s", html)
	}
	if !strings.Contains(html, `>Save item<`) {
		t.Fatalf("expected a Save item submit button, got:\n%s", html)
	}
}

func TestItemEditIsReachableFromTheDetailPage(t *testing.T) {
	t.Parallel()

	item := domain.Item{ID: "item-1", Name: "Rope"}
	detailHTML := renderItemDetailFromItem(t, item)
	editHTML := renderItemEdit(t, itemTemplateData(item))

	if !strings.Contains(detailHTML, "/items/item-1/edit") {
		t.Fatalf("expected the detail page to link to the edit page, got:\n%s", detailHTML)
	}
	if !strings.Contains(editHTML, `href="/items/item-1"`) {
		t.Fatalf("expected the edit page to link back to the detail page, got:\n%s", editHTML)
	}
}

func TestItemEditReusesLocationFieldsPartial(t *testing.T) {
	t.Parallel()

	item := domain.Item{ID: "item-1", Name: "Rope"}
	html := renderItemEdit(t, itemTemplateData(item))

	if got := strings.Count(html, `<select name="location_kind">`); got != 1 {
		t.Fatalf("expected location_kind select to appear once on the edit page, got %d in:\n%s", got, html)
	}
}

func TestItemDetailUsesDialogTriggerForSplitAndMerge(t *testing.T) {
	t.Parallel()

	data := itemTemplateData(domain.Item{ID: "item-1", Name: "Torches", IsStackable: true, Quantity: 5})
	data.ItemDetail.MergeTargets = []domain.Item{{ID: "item-2", Name: "Torches", Quantity: 3}}

	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	var rendered bytes.Buffer
	if err := server.tmpl.ExecuteTemplate(&rendered, "item_detail", data); err != nil {
		t.Fatalf("render item_detail: %v", err)
	}
	html := rendered.String()

	for _, want := range []string{
		`data-dialog-open="split-dialog"`,
		`id="split-dialog"`,
		`data-dialog-open="merge-dialog"`,
		`id="merge-dialog"`,
		`action="/items/item-1/split"`,
		`action="/items/item-1/merge"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected split/merge to use the dialog-trigger pattern via %q, got:\n%s", want, html)
		}
	}
	// The triggers must ship hidden, matching move/copy/delete's no-JS-then-reveal pattern.
	if !strings.Contains(html, `data-dialog-open="split-dialog" hidden`) {
		t.Fatalf("expected the split trigger to ship hidden, got:\n%s", html)
	}
}

func TestItemDetailOmitsSplitMergeDialogsWhenNotStackable(t *testing.T) {
	t.Parallel()

	html := renderItemDetailFromItem(t, domain.Item{ID: "item-1", Name: "Rope", IsStackable: false})

	if strings.Contains(html, `id="split-dialog"`) || strings.Contains(html, `id="merge-dialog"`) {
		t.Fatalf("expected no split/merge dialogs for a non-stackable item, got:\n%s", html)
	}
}
