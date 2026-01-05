package apierror

import "testing"

func TestSanitize_HidesInternalDetails(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "SQL error",
			input:    "pq: relation \"users\" does not exist",
			expected: "An internal error occurred",
		},
		{
			name:     "file path",
			input:    "open /var/lib/data/secret.json: no such file",
			expected: "An internal error occurred",
		},
		{
			name:     "connection error",
			input:    "dial tcp 192.168.1.100:5432: connection refused",
			expected: "An internal error occurred",
		},
		{
			name:     "safe validation error",
			input:    "name is required",
			expected: "name is required",
		},
		{
			name:     "safe format error",
			input:    "invalid date format",
			expected: "invalid date format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sanitize(tt.input)
			if got != tt.expected {
				t.Errorf("Sanitize(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
