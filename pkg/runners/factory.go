package runners

import (
	"fmt"

	"github.com/go-viper/mapstructure/v2"
	manifestcore "orch.io/pkg/manifest/core"
)

func FromManifest(name string, mr manifestcore.RunnerManifest) (Runner, error) {
	env, _, err := RetrieveProviderConfigForRunner(mr.Providers)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve provider config for runner %q: %w", name, err)
	}
	switch mr.Type {
	case "local":
		return &LocalRunner{name: name, env: env}, nil

	case "ssh":
		var cfg SSHRunnerConfig
		if err := mapstructure.Decode(mr.Config, &cfg); err != nil {
			return nil, fmt.Errorf("failed to decode ssh runner \"%s\" config: %w", name, err)
		}
		return &SSHRunner{name: name, config: cfg, env: env}, nil

	default:
		return nil, fmt.Errorf("unknown runner type: %s", mr.Type)
	}
}

func FromManifestRunnersMap(runnersMap map[string]manifestcore.RunnerManifest) (map[string]Runner, error) {
	runners := make(map[string]Runner)
	for name, mt := range runnersMap {
		runner, err := FromManifest(name, mt)
		if err != nil {
			return nil, fmt.Errorf("failed to create runner \"%s\": %w", name, err)
		}

		if err := runner.ValidateAndInitialize(); err != nil {
			return nil, fmt.Errorf("] runner \"%s\" failed to create: %w", name, err)
		}

		runners[name] = runner
	}
	return runners, nil
}
