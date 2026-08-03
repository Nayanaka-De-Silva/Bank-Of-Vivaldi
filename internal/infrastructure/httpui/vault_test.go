package httpui

import (
	"bytes"
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
	vault := domain.Vault{
		ID:              "vault-1",
		CharacterName:   "Aragorn",
		StrengthScore:   16,
		CarryModifierLB: 0,
		EncumbranceMode: domain.EncumbranceModeStandard,
		Notes:           "Ranger of the North",
		Purse:           domain.Purse{CP: 1, SP: 2, EP: 0, GP: 30, PP: 1},
	}

	detail := application.VaultDetail{
		Summary: application.VaultSummary{
			Vault:                      vault,
			TotalCarryWeightHundredths: 1000,
			MaxCarryWeightHundredths:   24000,
			EncumbranceState:           domain.EncumbranceStateNormal,
		},
		RootItems: []application.ItemNode{{
			Item: domain.Item{ID: "sword", Name: "Longsword", Category: "weapon"},
		}},
	}

	return TemplateData{
		Title:       "Aragorn",
		VaultDetail: detail,
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
