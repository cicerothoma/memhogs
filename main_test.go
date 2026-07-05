package main

import (
	"flag"
	"io"
	"reflect"
	"testing"
)

func TestParseFlagsAnywhere(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantFlat bool
		wantTop  int
		wantPos  []string
	}{
		{"flags before filter", []string{"--flat", "chrome"}, true, 0, []string{"chrome"}},
		{"flags after filter", []string{"chrome", "--flat"}, true, 0, []string{"chrome"}},
		{"value flag after filter", []string{"vtdecoder", "--top", "3"}, false, 3, []string{"vtdecoder"}},
		{"flags on both sides", []string{"--flat", "chrome", "--top", "2"}, true, 2, []string{"chrome"}},
		{"multi-word filter", []string{"google", "chrome", "--flat"}, true, 0, []string{"google", "chrome"}},
		{"no flags", []string{"chrome"}, false, 0, []string{"chrome"}},
		{"no args", nil, false, 0, nil},
		{"double dash protects flags", []string{"--", "--flat"}, false, 0, []string{"--flat"}},
		{"double dash after filter", []string{"chrome", "--", "--flat"}, false, 0, []string{"chrome", "--flat"}},
		{"lone dash is positional", []string{"chrome", "-"}, false, 0, []string{"chrome", "-"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("memhogs", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			flat := fs.Bool("flat", false, "")
			top := fs.Int("top", 0, "")
			got := parseFlagsAnywhere(fs, tc.args)
			if *flat != tc.wantFlat || *top != tc.wantTop {
				t.Errorf("flags = (flat=%v top=%d), want (flat=%v top=%d)", *flat, *top, tc.wantFlat, tc.wantTop)
			}
			if !reflect.DeepEqual(got, tc.wantPos) {
				t.Errorf("positionals = %q, want %q", got, tc.wantPos)
			}
		})
	}
}
