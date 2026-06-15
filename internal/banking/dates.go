package banking

import (
	"fmt"
	"strings"
	"time"
)

// ParseDateFormats parses bank statement dates commonly seen in Estonian exports.
func ParseDateFormats(dateStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"02.01.2006",
		"01/02/2006",
		"02/01/2006",
		"2006/01/02",
		"02-01-2006",
		"01-02-2006",
		time.RFC3339,
	}

	dateStr = strings.TrimSpace(dateStr)
	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}
