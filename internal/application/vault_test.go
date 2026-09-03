package application

import (
	"context"
	"testing"

	"bank-of-vivaldi/internal/domain"
)

func TestCreateVaultRequiresKind(t *testing.T) {
	t.Parallel()

	svc := NewService(newFakeStore())

	if _, err := svc.CreateVault(context.Background(), CreateVaultInput{
		CharacterName: "Nameless",
		StrengthScore: 10,
	}); err == nil {
		t.Fatal("expected an error when the vault kind is missing")
	}

	if _, err := svc.CreateVault(context.Background(), CreateVaultInput{
		CharacterName: "Bogus",
		StrengthScore: 10,
		Kind:          domain.VaultKind("monster"),
	}); err == nil {
		t.Fatal("expected an error when the vault kind is invalid")
	}

	vault, err := svc.CreateVault(context.Background(), CreateVaultInput{
		CharacterName: "The Lich",
		StrengthScore: 11,
		Kind:          domain.VaultKindNPC,
	})
	if err != nil {
		t.Fatalf("create npc vault: %v", err)
	}
	if vault.Kind != domain.VaultKindNPC {
		t.Fatalf("expected kind npc, got %q", vault.Kind)
	}
}

func TestUpdateVaultRequiresKind(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	svc := NewService(store)
	created, err := svc.CreateVault(context.Background(), CreateVaultInput{
		CharacterName: "Tordek",
		StrengthScore: 15,
		Kind:          domain.VaultKindPC,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := svc.UpdateVault(context.Background(), UpdateVaultInput{
		ID:            created.ID,
		CharacterName: "Tordek",
		StrengthScore: 15,
	}); err == nil {
		t.Fatal("expected an error when the vault kind is missing on update")
	}

	updated, err := svc.UpdateVault(context.Background(), UpdateVaultInput{
		ID:            created.ID,
		CharacterName: "Tordek",
		StrengthScore: 15,
		Kind:          domain.VaultKindNPC,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Kind != domain.VaultKindNPC {
		t.Fatalf("expected kind npc after update, got %q", updated.Kind)
	}
}

func TestDashboardHonoursKindFilter(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	svc := NewService(store)
	for _, in := range []CreateVaultInput{
		{CharacterName: "Hero", StrengthScore: 10, Kind: domain.VaultKindPC},
		{CharacterName: "Goblin", StrengthScore: 8, Kind: domain.VaultKindNPC},
	} {
		if _, err := svc.CreateVault(context.Background(), in); err != nil {
			t.Fatalf("seed %s: %v", in.CharacterName, err)
		}
	}

	data, err := svc.Dashboard(context.Background(), VaultListFilters{Kind: "npc"})
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if len(data.Vaults) != 1 || data.Vaults[0].Vault.CharacterName != "Goblin" {
		t.Fatalf("expected only the Goblin NPC vault, got %v", data.Vaults)
	}
}
