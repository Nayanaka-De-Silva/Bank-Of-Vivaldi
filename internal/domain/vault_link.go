package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	// MaxExternalRefLength caps the caller-owned identifier so a rogue client
	// cannot bloat the index on (vault_id, external_ref).
	MaxExternalRefLength = 200

	// MaxLinkLabelLength caps the human-readable description of a link.
	MaxLinkLabelLength = 200
)

// VaultLink records that an external application has registered an interest in
// a vault. It grants no ownership: unlinking removes only this row, never the
// vault itself.
type VaultLink struct {
	ID          string
	VaultID     string
	ExternalRef string // caller-owned identifier, e.g. "npc-manager:char-42"
	Label       string
	CreatedAt   time.Time
}

// Normalize returns a copy with surrounding whitespace trimmed from the
// caller-supplied text fields.
func (l VaultLink) Normalize() VaultLink {
	l.VaultID = strings.TrimSpace(l.VaultID)
	l.ExternalRef = strings.TrimSpace(l.ExternalRef)
	l.Label = strings.TrimSpace(l.Label)
	return l
}

func (l VaultLink) Validate() error {
	link := l.Normalize()

	if link.VaultID == "" {
		return fmt.Errorf("vault id is required: %w", ErrInvalidInput)
	}
	if link.ExternalRef == "" {
		return fmt.Errorf("external reference is required: %w", ErrInvalidInput)
	}
	if len(link.ExternalRef) > MaxExternalRefLength {
		return fmt.Errorf("external reference must be at most %d characters: %w", MaxExternalRefLength, ErrInvalidInput)
	}
	if len(link.Label) > MaxLinkLabelLength {
		return fmt.Errorf("label must be at most %d characters: %w", MaxLinkLabelLength, ErrInvalidInput)
	}
	return nil
}
