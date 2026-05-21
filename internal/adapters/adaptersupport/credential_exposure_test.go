package adaptersupport

import (
	"strings"
	"testing"

	manifestcore "orch.io/pkg/manifest/core"
)

func TestDetectCredentialExposureEnv(t *testing.T) {
	got := detectCredentialExposureEnv(map[string]string{
		"AWS_ACCESS_KEY_ID":     "key",
		"aws_secret_access_key": "secret",
		"AWS_PROFILE":           "profile",
		"APP_TOKEN":             "app-token",
		"DATABASE_PASSWORD":     "password",
		"PLAIN_VALUE":           "plain",
	})

	want := []string{"env.APP_TOKEN", "env.AWS_ACCESS_KEY_ID", "env.DATABASE_PASSWORD", "env.aws_secret_access_key"}
	if len(got) != len(want) {
		t.Fatalf("credential refs = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("credential refs = %#v, want %#v", got, want)
		}
	}
}

func TestCredentialExposureWarningDoesNotIncludeValues(t *testing.T) {
	event := credentialExposureWarning(&manifestcore.Component{
		Name:   "infra",
		Type:   "terraform",
		Runner: "local",
	}, []string{"env.AWS_SECRET_ACCESS_KEY"})

	if !strings.Contains(event.Message, "env.AWS_SECRET_ACCESS_KEY") {
		t.Fatalf("warning message does not include credential ref: %q", event.Message)
	}
	if strings.Contains(event.Message, "secret-value") {
		t.Fatalf("warning message should not include credential value: %q", event.Message)
	}
	if event.Component != "infra" || event.Adapter != "terraform" || event.Runner != "local" {
		t.Fatalf("unexpected event metadata: %#v", event)
	}
}

func TestCredentialExposureWarningsIncludesComponentEnv(t *testing.T) {
	warnings := CredentialExposureWarnings(&manifestcore.Component{
		Name:   "infra",
		Type:   "cloudformation",
		Runner: "local",
		Env: map[string]string{
			"AWS_ACCESS_KEY_ID": "component-key",
			"DEPLOY_TOKEN":      "deploy-token",
		},
	})

	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got %d", len(warnings))
	}
	for _, want := range []string{"env.AWS_ACCESS_KEY_ID", "env.DEPLOY_TOKEN"} {
		if !strings.Contains(warnings[0].Message, want) {
			t.Fatalf("warning missing %q: %s", want, warnings[0].Message)
		}
	}
	for _, leaked := range []string{"component-key", "secret"} {
		if strings.Contains(warnings[0].Message, leaked) {
			t.Fatalf("warning leaked value %q: %s", leaked, warnings[0].Message)
		}
	}
}
