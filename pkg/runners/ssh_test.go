package runners

import "testing"

func TestBuildSSHCommandIncludesWorkDirAndEnv(t *testing.T) {
	got := buildSSHCommand(
		map[string]string{"BASE": "from-base", "OVERRIDE": "base"},
		ExecCommand{
			WorkingDir: "/tmp/orch app",
			Command:    []string{"sh", "-c", "printf '%s' \"$TOKEN\""},
			Env: map[string]string{
				"OVERRIDE": "request",
				"TOKEN":    "abc 123",
			},
		},
	)

	want := "cd '/tmp/orch app' && env 'BASE=from-base' 'OVERRIDE=request' 'TOKEN=abc 123' 'sh' '-c' 'printf '\\''%s'\\'' \"$TOKEN\"'"
	if got != want {
		t.Fatalf("command mismatch\nwant: %s\n got: %s", want, got)
	}
}

func TestShellQuote(t *testing.T) {
	got := shellQuote("can't stop")
	want := "'can'\\''t stop'"
	if got != want {
		t.Fatalf("quote mismatch: want %s, got %s", want, got)
	}
}
