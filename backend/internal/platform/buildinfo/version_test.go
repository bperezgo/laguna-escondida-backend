package buildinfo

import "testing"

func TestVersionLess(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"v1.1.9", "v1.2.0", true},
		{"v1.2.0", "v1.2.0", false},
		{"v1.10.0", "v1.2.0", false},
		{"v1.2.0-rc1", "v1.2.1", true},
		{"dev", "v1.2.0", false},
		{"", "v1.2.0", false},
		{"v1.2.0", "dev", false},
	}
	for _, tt := range tests {
		if got := VersionLess(tt.a, tt.b); got != tt.want {
			t.Errorf("VersionLess(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
