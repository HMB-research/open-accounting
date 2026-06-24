package workspace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemediationAssignment(t *testing.T) {
	meta := RemediationAssignment(" Tax Declarations ", "KMD Payable Review", "ACTION", "KMD", "2026-03")

	assert.Equal(t, "Tax Declarations", meta.WorkspaceQueue)
	assert.Equal(t, "tax-declarations:kmd-payable-review:kmd:2026-03", meta.AssignmentKey)
	assert.Equal(t, PriorityHigh, meta.Priority)
	assert.Equal(t, 1, meta.DueInDays)
}

func TestAssignmentKeyUsesPlaceholderWhenPartsAreEmpty(t *testing.T) {
	assert.Equal(t, "documents:missing-evidence:-", AssignmentKey("documents", "missing evidence"))
}

func TestAssignmentPriority(t *testing.T) {
	tests := []struct {
		severity string
		priority string
		due      int
	}{
		{severity: "BLOCKER", priority: PriorityHigh, due: 1},
		{severity: "ACTION", priority: PriorityHigh, due: 1},
		{severity: "ERROR", priority: PriorityHigh, due: 1},
		{severity: "WARNING", priority: PriorityNormal, due: 3},
		{severity: "INFO", priority: PriorityLow, due: 0},
		{severity: "unknown", priority: PriorityNormal, due: 3},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			priority, due := AssignmentPriority(tt.severity)
			assert.Equal(t, tt.priority, priority)
			assert.Equal(t, tt.due, due)
		})
	}
}

func TestNormalizeAssignmentPart(t *testing.T) {
	assert.Equal(t, "migration-cutover", NormalizeAssignmentPart(" Migration / Cutover "))
	assert.Equal(t, "kmd-2026-03", NormalizeAssignmentPart("KMD 2026.03"))
	assert.Equal(t, "-", NormalizeAssignmentPart(" / "))
	assert.Equal(t, "a-b", NormalizeAssignmentPart("a---b"))
}
