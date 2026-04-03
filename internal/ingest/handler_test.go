package ingest

import "testing"

func TestNormalizeIssueLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		level          string
		warningAsError bool
		want           string
	}{
		{name: "warning promoted", level: "warning", warningAsError: true, want: "error"},
		{name: "warning unchanged", level: "warning", warningAsError: false, want: "warning"},
		{name: "error unchanged", level: "error", warningAsError: true, want: "error"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := normalizeIssueLevel(tt.level, tt.warningAsError); got != tt.want {
				t.Fatalf("normalizeIssueLevel(%q, %t) = %q, want %q", tt.level, tt.warningAsError, got, tt.want)
			}
		})
	}
}
