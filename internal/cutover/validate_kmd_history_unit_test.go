package cutover

import "testing"

func TestKMDHistoryTotalRowCode(t *testing.T) {
	tests := []struct {
		label string
		want  string
	}{
		{label: "input", want: "9"},
		{label: "output", want: "8"},
		{label: "", want: "8"},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			if got := kmdHistoryTotalRowCode(tt.label); got != tt.want {
				t.Fatalf("kmdHistoryTotalRowCode(%q) = %q, want %q", tt.label, got, tt.want)
			}
		})
	}
}
