package proc

import "testing"

func TestParseMemTotal(t *testing.T) {
	tests := []struct {
		meminfo string
		want    uint64
	}{
		{"MemTotal:       65486788 kB\nMemFree:        1234 kB\n", 65486788 * 1024},
		{"MemFree:        1234 kB\n", 0},
		{"MemTotal:       garbage kB\n", 0},
		{"", 0},
	}
	for _, tt := range tests {
		if got := parseMemTotal(tt.meminfo); got != tt.want {
			t.Errorf("parseMemTotal(%q) = %d, want %d", tt.meminfo, got, tt.want)
		}
	}
}
