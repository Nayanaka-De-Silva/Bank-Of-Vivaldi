package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestVaultLinkValidate(t *testing.T) {
	tests := []struct {
		name    string
		link    VaultLink
		wantErr bool
	}{
		{
			name: "valid link",
			link: VaultLink{VaultID: "vault-1", ExternalRef: "npc-manager:char-42"},
		},
		{
			name:    "missing vault id",
			link:    VaultLink{ExternalRef: "npc-manager:char-42"},
			wantErr: true,
		},
		{
			name:    "missing external ref",
			link:    VaultLink{VaultID: "vault-1"},
			wantErr: true,
		},
		{
			name:    "blank external ref",
			link:    VaultLink{VaultID: "vault-1", ExternalRef: "   "},
			wantErr: true,
		},
		{
			name:    "external ref too long",
			link:    VaultLink{VaultID: "vault-1", ExternalRef: strings.Repeat("a", MaxExternalRefLength+1)},
			wantErr: true,
		},
		{
			name: "external ref at max length",
			link: VaultLink{VaultID: "vault-1", ExternalRef: strings.Repeat("a", MaxExternalRefLength)},
		},
		{
			name:    "label too long",
			link:    VaultLink{VaultID: "vault-1", ExternalRef: "ref", Label: strings.Repeat("a", MaxLinkLabelLength+1)},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.link.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			// Validation failures must be classifiable as bad input by callers.
			if tc.wantErr && !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected error to wrap ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestVaultLinkNormalizeTrimsWhitespace(t *testing.T) {
	link := VaultLink{
		VaultID:     "  vault-1  ",
		ExternalRef: "  npc-manager:char-42  ",
		Label:       "  Bruenor's pack  ",
	}

	normalized := link.Normalize()

	if normalized.VaultID != "vault-1" {
		t.Fatalf("expected trimmed vault id, got %q", normalized.VaultID)
	}
	if normalized.ExternalRef != "npc-manager:char-42" {
		t.Fatalf("expected trimmed external ref, got %q", normalized.ExternalRef)
	}
	if normalized.Label != "Bruenor's pack" {
		t.Fatalf("expected trimmed label, got %q", normalized.Label)
	}
}
