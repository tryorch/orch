package runners

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	manifestcore "orch.io/pkg/manifest/core"
)

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

func TestSSHHostKeyCallbackRequiresVerificationMethod(t *testing.T) {
	runner := &SSHRunner{}
	_, err := runner.hostKeyCallback()
	if err == nil {
		t.Fatal("expected missing host key config to fail")
	}
	if !strings.Contains(err.Error(), "host_key is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSSHHostKeyCallbackRejectsMultipleVerificationMethods(t *testing.T) {
	runner := &SSHRunner{}
	runner.config.HostKey.KnownHosts = filepath.Join(t.TempDir(), "known_hosts")
	runner.config.HostKey.Insecure = true

	_, err := runner.hostKeyCallback()
	if err == nil {
		t.Fatal("expected multiple host key methods to fail")
	}
	if !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSSHHostKeyCallbackAllowsExplicitInsecure(t *testing.T) {
	runner := &SSHRunner{}
	runner.config.HostKey.Insecure = true

	callback, err := runner.hostKeyCallback()
	if err != nil {
		t.Fatalf("expected insecure host key callback: %v", err)
	}
	if callback == nil {
		t.Fatal("expected callback")
	}
}

func TestSSHHostKeyCallbackUsesKnownHosts(t *testing.T) {
	pubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	sshKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		t.Fatalf("failed to create ssh public key: %v", err)
	}

	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	line := knownhosts.Line([]string{"example.com"}, sshKey)
	if err := os.WriteFile(knownHostsPath, []byte(line+"\n"), 0644); err != nil {
		t.Fatalf("failed to write known_hosts: %v", err)
	}

	runner := &SSHRunner{}
	runner.config.HostKey.KnownHosts = knownHostsPath
	callback, err := runner.hostKeyCallback()
	if err != nil {
		t.Fatalf("expected known_hosts callback: %v", err)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("203.0.113.10"), Port: 22}
	if err := callback("example.com:22", addr, sshKey); err != nil {
		t.Fatalf("expected known host to verify: %v", err)
	}
}

func TestExpandUserPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to resolve home: %v", err)
	}

	got, err := expandUserPath("~/.ssh/known_hosts")
	if err != nil {
		t.Fatalf("expand failed: %v", err)
	}
	want := filepath.Join(home, ".ssh", "known_hosts")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFromManifestDecodesSSHHostKeyConfig(t *testing.T) {
	runner, err := FromManifest("remote", manifestcore.RunnerManifest{
		Type: "ssh",
		Config: map[string]interface{}{
			"host": "example.com",
			"port": 22,
			"user": "deploy",
			"auth": map[string]interface{}{
				"method":   "password",
				"password": "secret",
			},
			"host_key": map[string]interface{}{
				"insecure": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("from manifest failed: %v", err)
	}

	sshRunner, ok := runner.(*SSHRunner)
	if !ok {
		t.Fatalf("runner type = %T", runner)
	}
	if !sshRunner.config.HostKey.Insecure {
		t.Fatal("expected host_key.insecure to decode")
	}
}
