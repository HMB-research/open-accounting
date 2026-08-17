package tenant

import (
	"fmt"
	"strings"
)

const (
	// EvidencePolicyModeWarn preserves the existing opt-in evidence behavior.
	EvidencePolicyModeWarn = "warn"
	// EvidencePolicyModeBlockHighRisk requires approved evidence for pilot high-risk workflows.
	EvidencePolicyModeBlockHighRisk = "block_high_risk"
)

// NormalizeEvidencePolicyMode returns the persisted canonical policy mode.
func NormalizeEvidencePolicyMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", EvidencePolicyModeWarn:
		return EvidencePolicyModeWarn, nil
	case EvidencePolicyModeBlockHighRisk:
		return EvidencePolicyModeBlockHighRisk, nil
	default:
		return "", fmt.Errorf("invalid evidence policy mode %q", value)
	}
}

// BlocksHighRiskEvidence reports whether the tenant must block pilot high-risk mutations
// without approved evidence. Invalid legacy values deliberately fail open to the compatible
// warn behavior; service writes reject invalid new values through normalization.
func (s TenantSettings) BlocksHighRiskEvidence() bool {
	return strings.EqualFold(strings.TrimSpace(s.EvidencePolicyMode), EvidencePolicyModeBlockHighRisk)
}
