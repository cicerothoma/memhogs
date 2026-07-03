package proc

import "testing"

func TestParsePss(t *testing.T) {
	rollup := "00400000-7fff Rollup\nRss:            123456 kB\nPss:             78901 kB\nPss_Anon:        50000 kB\n"
	if got := parsePss(rollup); got != 78901*1024 {
		t.Errorf("parsePss = %d, want %d", got, 78901*1024)
	}
	if got := parsePss("Rss: 5 kB\n"); got != 0 {
		t.Errorf("parsePss without Pss line = %d, want 0", got)
	}
}

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
