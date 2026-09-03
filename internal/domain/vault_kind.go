package domain

import "strings"

// VaultKind classifies a vault as belonging to a Player Character or NPC.
// There is no default: callers must always supply an explicit kind.
type VaultKind string

const (
	VaultKindPC  VaultKind = "pc"
	VaultKindNPC VaultKind = "npc"
)

// Valid reports whether k is a recognised vault kind.
func (k VaultKind) Valid() bool {
	return k == VaultKindPC || k == VaultKindNPC
}

// ParseVaultKind is a strict parser: it returns the VaultKind and true only
// when value is a recognised kind string. Unlike ParseEncumbranceMode it has
// no fallback — there is no sensible default.
func ParseVaultKind(value string) (VaultKind, bool) {
	normalized := VaultKind(strings.ToLower(strings.TrimSpace(value)))
	switch normalized {
	case VaultKindPC, VaultKindNPC:
		return normalized, true
	default:
		return "", false
	}
}

// Label returns the display label for the vault kind. HumanizeLabel would
// wrongly produce "Pc", so we return uppercase abbreviations directly.
func (k VaultKind) Label() string {
	switch k {
	case VaultKindPC:
		return "PC"
	case VaultKindNPC:
		return "NPC"
	default:
		return string(k)
	}
}

// VaultKinds returns the ordered list of valid vault kinds for option lists.
func VaultKinds() []VaultKind {
	return []VaultKind{VaultKindPC, VaultKindNPC}
}
