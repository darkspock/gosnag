package issue

import "testing"

func TestResolveCooldownMinutes(t *testing.T) {
	t.Parallel()

	explicitNone := 0
	explicitCustom := 15

	tests := []struct {
		name           string
		projectDefault int32
		requested      *int
		want           *int
	}{
		{name: "uses explicit value", projectDefault: 30, requested: &explicitCustom, want: &explicitCustom},
		{name: "uses explicit none", projectDefault: 30, requested: &explicitNone, want: &explicitNone},
		{name: "falls back to project default", projectDefault: 45, requested: nil, want: intPtr(45)},
		{name: "no fallback when project default disabled", projectDefault: 0, requested: nil, want: nil},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := resolveCooldownMinutes(tt.projectDefault, tt.requested)
			switch {
			case got == nil && tt.want == nil:
				return
			case got == nil || tt.want == nil:
				t.Fatalf("resolveCooldownMinutes(%d, %v) = %v, want %v", tt.projectDefault, tt.requested, got, tt.want)
			case *got != *tt.want:
				t.Fatalf("resolveCooldownMinutes(%d, %v) = %d, want %d", tt.projectDefault, tt.requested, *got, *tt.want)
			}
		})
	}
}

func intPtr(v int) *int {
	return &v
}
