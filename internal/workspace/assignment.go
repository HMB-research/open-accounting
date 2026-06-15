package workspace

import "strings"

const (
	PriorityHigh   = "high"
	PriorityNormal = "normal"
	PriorityLow    = "low"
)

// AssignmentMetadata is the common accountant-workspace routing metadata for remediation actions.
type AssignmentMetadata struct {
	WorkspaceQueue string
	AssignmentKey  string
	Priority       string
	DueInDays      int
}

// RemediationAssignment returns stable assignment routing for a remediation action.
func RemediationAssignment(queue, code, severity string, parts ...string) AssignmentMetadata {
	priority, dueInDays := AssignmentPriority(severity)
	return AssignmentMetadata{
		WorkspaceQueue: strings.TrimSpace(queue),
		AssignmentKey:  AssignmentKey(queue, code, parts...),
		Priority:       priority,
		DueInDays:      dueInDays,
	}
}

// AssignmentPriority maps remediation severity to workspace priority and due-window defaults.
func AssignmentPriority(severity string) (string, int) {
	switch strings.ToUpper(strings.TrimSpace(severity)) {
	case "BLOCKER", "ACTION", "ERROR":
		return PriorityHigh, 1
	case "WARNING":
		return PriorityNormal, 3
	case "INFO":
		return PriorityLow, 0
	default:
		return PriorityNormal, 3
	}
}

// AssignmentKey builds deterministic, normalized queue keys.
func AssignmentKey(queue, code string, parts ...string) string {
	segments := []string{
		NormalizeAssignmentPart(queue),
		NormalizeAssignmentPart(code),
	}
	if len(parts) == 0 {
		segments = append(segments, "-")
	}
	for _, part := range parts {
		segments = append(segments, NormalizeAssignmentPart(part))
	}
	return strings.Join(segments, ":")
}

// NormalizeAssignmentPart keeps assignment keys stable across whitespace and punctuation changes.
func NormalizeAssignmentPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	normalized := strings.Trim(builder.String(), "-")
	if normalized == "" {
		return "-"
	}
	return normalized
}
