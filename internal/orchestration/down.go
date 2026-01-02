package orchestration

import (
	"context"
	"fmt"

	"orch.io/internal/adapters"
	"orch.io/pkg/logging"
	manifestcore "orch.io/pkg/manifest/core"
	"orch.io/pkg/runners"
)

func RunDown(m *manifestcore.Manifest, logger logging.Logger) error {
	fmt.Printf("Tearing down sandbox: %s\n", m.Metadata.ID)

	allRunners, err := runners.FromManifestRunnersMap(m.Runners)
	if err != nil {
		return fmt.Errorf("failed to create runners from manifest: %w", err)
	}

	for i := range m.Components {
		c := &m.Components[i]
		t, ok := allRunners[c.Runner]
		if !ok {
			return fmt.Errorf("component \"%s\" references unknown runner \"%s\"",
				c.Name, c.Runner)
		}
		fmt.Printf("→ Destroying component: %s\n", c.Name)
		adapter, err := adapters.Get(c.Type)
		if err != nil {
			return err
		}

		cfg, _, err := adapter.ValidateAndLoadConfig(context.Background(), c)
		if err != nil {
			return fmt.Errorf("component \"%s\" config validation failed: %w", c.Name, err)
		}

		c.LoadedConfig = cfg

		if err := adapter.Destroy(context.TODO(), c, t); err != nil {
			return fmt.Errorf("component %s destroy failed: %w", c.Name, err)
		}
	}

	fmt.Printf("🧹 Sandbox torn down\n")
	return nil
}
