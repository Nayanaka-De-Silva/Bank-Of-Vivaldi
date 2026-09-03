package application

import (
	"context"
	"errors"
	"testing"

	"bank-of-vivaldi/internal/domain"
)

func newVaultForLinking(t *testing.T, svc *Service) domain.Vault {
	t.Helper()
	vault, err := svc.CreateVault(context.Background(), CreateVaultInput{
		CharacterName: "Bruenor",
		StrengthScore: 16,
		Kind:          domain.VaultKindPC,
	})
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}
	return vault
}

func TestLinkVaultCreatesLink(t *testing.T) {
	svc := NewService(newFakeStore())
	vault := newVaultForLinking(t, svc)

	link, err := svc.LinkVault(context.Background(), LinkVaultInput{
		VaultID:     vault.ID,
		ExternalRef: "npc-manager:char-42",
		Label:       "Bruenor's pack",
	})
	if err != nil {
		t.Fatalf("link vault: %v", err)
	}

	if link.ID == "" {
		t.Fatalf("expected generated link id")
	}
	if link.VaultID != vault.ID {
		t.Fatalf("expected vault id %q, got %q", vault.ID, link.VaultID)
	}
	if link.CreatedAt.IsZero() {
		t.Fatalf("expected CreatedAt to be stamped")
	}

	links, err := svc.ListVaultLinks(context.Background(), vault.ID)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
}

func TestLinkVaultTrimsInput(t *testing.T) {
	svc := NewService(newFakeStore())
	vault := newVaultForLinking(t, svc)

	link, err := svc.LinkVault(context.Background(), LinkVaultInput{
		VaultID:     vault.ID,
		ExternalRef: "  npc-manager:char-42  ",
		Label:       "  Bruenor's pack  ",
	})
	if err != nil {
		t.Fatalf("link vault: %v", err)
	}

	if link.ExternalRef != "npc-manager:char-42" {
		t.Fatalf("expected trimmed external ref, got %q", link.ExternalRef)
	}
	if link.Label != "Bruenor's pack" {
		t.Fatalf("expected trimmed label, got %q", link.Label)
	}
}

func TestLinkVaultIsIdempotent(t *testing.T) {
	svc := NewService(newFakeStore())
	vault := newVaultForLinking(t, svc)
	input := LinkVaultInput{VaultID: vault.ID, ExternalRef: "npc-manager:char-42"}

	first, err := svc.LinkVault(context.Background(), input)
	if err != nil {
		t.Fatalf("first link: %v", err)
	}
	second, err := svc.LinkVault(context.Background(), input)
	if err != nil {
		t.Fatalf("relinking the same ref should not error: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("expected the existing link to be returned, got %q then %q", first.ID, second.ID)
	}

	links, _ := svc.ListVaultLinks(context.Background(), vault.ID)
	if len(links) != 1 {
		t.Fatalf("expected relinking to not duplicate, got %d links", len(links))
	}
}

func TestLinkVaultRejectsUnknownVault(t *testing.T) {
	svc := NewService(newFakeStore())

	_, err := svc.LinkVault(context.Background(), LinkVaultInput{
		VaultID:     "does-not-exist",
		ExternalRef: "npc-manager:char-42",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestLinkVaultRejectsInvalidInput(t *testing.T) {
	svc := NewService(newFakeStore())
	vault := newVaultForLinking(t, svc)

	_, err := svc.LinkVault(context.Background(), LinkVaultInput{VaultID: vault.ID, ExternalRef: "  "})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestUnlinkVaultRemovesOnlyTheLink(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)
	vault := newVaultForLinking(t, svc)

	if _, err := svc.LinkVault(context.Background(), LinkVaultInput{VaultID: vault.ID, ExternalRef: "a"}); err != nil {
		t.Fatalf("link a: %v", err)
	}
	if _, err := svc.LinkVault(context.Background(), LinkVaultInput{VaultID: vault.ID, ExternalRef: "b"}); err != nil {
		t.Fatalf("link b: %v", err)
	}

	if err := svc.UnlinkVault(context.Background(), vault.ID, "a"); err != nil {
		t.Fatalf("unlink: %v", err)
	}

	links, _ := svc.ListVaultLinks(context.Background(), vault.ID)
	if len(links) != 1 || links[0].ExternalRef != "b" {
		t.Fatalf("expected only link %q to remain, got %+v", "b", links)
	}

	// The vault itself must survive an unlink.
	if _, err := store.GetVault(context.Background(), vault.ID); err != nil {
		t.Fatalf("unlink must not remove the vault: %v", err)
	}
}

func TestUnlinkVaultUnknownRefIsNotFound(t *testing.T) {
	svc := NewService(newFakeStore())
	vault := newVaultForLinking(t, svc)

	err := svc.UnlinkVault(context.Background(), vault.ID, "never-linked")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
