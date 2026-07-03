package proc

import "testing"

func TestParsePS(t *testing.T) {
	out := []byte(`    1     0   9184 /sbin/launchd
  200     1 819200 /Applications/Warp.app/Contents/MacOS/stable
  201   200  10240 -zsh
  302   301 119808 /usr/local/bin/node
 9999     1      0 /usr/libexec/idle
garbage line here
`)
	procs, err := parsePS(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(procs) != 5 {
		t.Fatalf("got %d procs, want 5: %+v", len(procs), procs)
	}

	warp := procs[1]
	if warp.PID != 200 || warp.PPID != 1 || warp.RSS != 819200*1024 {
		t.Errorf("warp fields wrong: %+v", warp)
	}
	if warp.Path != "/Applications/Warp.app/Contents/MacOS/stable" || warp.Name != "stable" {
		t.Errorf("warp path/name wrong: %+v", warp)
	}

	zsh := procs[2]
	if zsh.Path != "" || zsh.Name != "-zsh" {
		t.Errorf("login shell should keep bare comm as name: %+v", zsh)
	}
}
