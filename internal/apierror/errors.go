package apierror

import (
	"regexp"
	"strings"
)

// Patterns that indicate internal/sensitive errors
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)pq:|pgx:|sql:|postgres`),
	regexp.MustCompile(`(?i)connection|timeout|refused`),
	regexp.MustCompile(`(?i)/var/|/tmp/|/home/|/app/|\.go:\d+`),
	regexp.MustCompile(`(?i)dial tcp|network|socket`),
	regexp.MustCompile(`(?i)panic|runtime error`),
	regexp.MustCompile(`(?i)internal server|stack trace`),
	regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`), // IP addresses
}

const genericError = "An internal error occurred"

// Sanitize removes sensitive information from error messages
// Safe messages (validation errors, format errors) are passed through
func Sanitize(msg string) string {
	for _, pattern := range sensitivePatterns {
		if pattern.MatchString(msg) {
			return genericError
		}
	}

	// Additional check for file paths
	if strings.Contains(msg, "/") && (strings.Contains(msg, "open") || strings.Contains(msg, "read") || strings.Contains(msg, "write")) {
		return genericError
	}

	return msg
}
