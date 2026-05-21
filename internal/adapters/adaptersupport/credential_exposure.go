package adaptersupport

import (
	"fmt"
	"sort"
	"strings"

	"orch.io/pkg/events"
	manifestcore "orch.io/pkg/manifest/core"
)

var explicitProviderEnvKeys = map[string]struct{}{
	"ARM_ACCESS_KEY":                        {},
	"ARM_CLIENT_SECRET":                     {},
	"AWS_ACCESS_KEY_ID":                     {},
	"AWS_SECRET_ACCESS_KEY":                 {},
	"AWS_SESSION_TOKEN":                     {},
	"AZURE_CLIENT_CERTIFICATE_PASSWORD":     {},
	"AZURE_CLIENT_CERTIFICATE_PATH":         {},
	"AZURE_CLIENT_SECRET":                   {},
	"CLOUDFLARE_API_KEY":                    {},
	"CLOUDFLARE_API_TOKEN":                  {},
	"DIGITALOCEAN_ACCESS_TOKEN":             {},
	"DIGITALOCEAN_TOKEN":                    {},
	"GITHUB_TOKEN":                          {},
	"GITLAB_TOKEN":                          {},
	"GOOGLE_APPLICATION_CREDENTIALS":        {},
	"GOOGLE_APPLICATION_CREDENTIALS_JSON":   {},
	"GOOGLE_BACKEND_CREDENTIALS":            {},
	"GOOGLE_CREDENTIALS":                    {},
	"GOOGLE_IMPERSONATE_SERVICE_ACCOUNT":    {},
	"TF_VAR_AWS_ACCESS_KEY_ID":              {},
	"TF_VAR_AWS_SECRET_ACCESS_KEY":          {},
	"TF_VAR_AWS_SESSION_TOKEN":              {},
	"TF_VAR_GOOGLE_APPLICATION_CREDENTIALS": {},
}

var credentialKeyParts = []string{
	"access_key",
	"api_key",
	"credential",
	"password",
	"passwd",
	"private_key",
	"secret",
	"token",
}

func CredentialExposureWarnings(c *manifestcore.Component) []events.Event {
	refs := detectCredentialExposureEnv(c.Env)

	if len(refs) == 0 {
		return nil
	}
	return []events.Event{credentialExposureWarning(c, refs)}
}

func detectCredentialExposureEnv(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	return detectCredentialExposureKeys("env", keys)
}

func detectCredentialExposureKeys(prefix string, keys []string) []string {
	var refs []string
	for _, key := range keys {
		if looksLikeCredentialKey(key) {
			refs = append(refs, prefix+"."+key)
		}
	}
	sort.Strings(refs)
	return refs
}

func looksLikeCredentialKey(key string) bool {
	normalized := strings.ToUpper(key)
	if _, ok := explicitProviderEnvKeys[normalized]; ok {
		return true
	}

	lower := strings.ToLower(key)
	for _, part := range credentialKeyParts {
		if strings.Contains(lower, part) {
			return true
		}
	}
	return false
}

func credentialExposureWarning(c *manifestcore.Component, refs []string) events.Event {
	sort.Strings(refs)
	return events.Event{
		Type: events.EventWarning,
		Message: fmt.Sprintf(
			"Component environment contains keys that look like access mechanisms (%s). Orch will pass these values to runner processes.",
			strings.Join(refs, ", "),
		),
		Hint:      "Prefer ambient auth, runner-local secret injection, or short-lived environment values when possible.",
		Adapter:   c.Type,
		Runner:    c.Runner,
		Component: c.Name,
	}
}
