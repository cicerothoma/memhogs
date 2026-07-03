package group

import (
	"testing"

	"github.com/cicerothoma/memhogs/internal/proc"
)

func TestOutermostBundle(t *testing.T) {
	tests := []struct {
		path string
		want string
		ok   bool
	}{
		{"/Applications/Warp.app/Contents/MacOS/stable", "Warp", true},
		{"/Applications/Cursor.app/Contents/Frameworks/Cursor Helper (Plugin).app/Contents/MacOS/Cursor Helper (Plugin)", "Cursor", true},
		{"/System/Applications/Utilities/Terminal.app/Contents/MacOS/Terminal", "Terminal", true},
		{"/Users/cicero/Applications/Foo.app/Contents/MacOS/Foo", "Foo", true},
		{"/usr/local/bin/node", "", false},
		{"", "", false},
		{"/opt/weird/.app/bin/x", "", false}, // bare ".app" segment is not a bundle
	}
	for _, tt := range tests {
		got, ok := outermostBundle(tt.path)
		if got != tt.want || ok != tt.ok {
			t.Errorf("outermostBundle(%q) = %q,%v want %q,%v", tt.path, got, ok, tt.want, tt.ok)
		}
	}
}

func TestIsShell(t *testing.T) {
	shell := []proc.Proc{
		{Path: "/bin/zsh", Name: "-zsh"},
		{Path: "/bin/bash", Name: "bash"},
		{Path: "", Name: "-fish"},
		{Path: "/usr/bin/dash", Name: "dash"},
	}
	notShell := []proc.Proc{
		{Path: "/usr/local/bin/node", Name: "node"},
		{Path: "/Applications/Warp.app/Contents/MacOS/stable", Name: "stable"},
		{Path: "/usr/bin/ssh", Name: "ssh"},
	}
	for _, p := range shell {
		if !isShell(p) {
			t.Errorf("isShell(%q/%q) = false, want true", p.Path, p.Name)
		}
	}
	for _, p := range notShell {
		if isShell(p) {
			t.Errorf("isShell(%q/%q) = true, want false", p.Path, p.Name)
		}
	}
}

func TestUnitIdentity(t *testing.T) {
	tests := []struct {
		unit string
		want string
		ok   bool
	}{
		{"app-gnome-firefox-4321.scope", "firefox", true},
		{"app-gnome-org.gnome.Terminal-2172.scope", "Terminal", true},
		{"app-flatpak-com.spotify.Client-99.scope", "Client", true},
		{"ssh.service", "ssh", true},
		{"session-3.scope", "", false},
		{"init.scope", "", false},
		{"user@1000.service", "", false},
		{"-.slice", "", false},
		{"app.slice", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := unitIdentity(tt.unit)
		if got != tt.want || ok != tt.ok {
			t.Errorf("unitIdentity(%q) = %q,%v want %q,%v", tt.unit, got, ok, tt.want, tt.ok)
		}
	}
}
