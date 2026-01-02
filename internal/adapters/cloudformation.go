package adapters

import (
	"context"

	"orch.io/pkg/events"
	manifestcore "orch.io/pkg/manifest/core"
	"orch.io/pkg/runners"
)

type CloudFormationAdapter struct{}

func (d *CloudFormationAdapter) RequiredCapabilities() runners.Capabilities {
	return runners.Capabilities{API: true}
}

func (d *CloudFormationAdapter) SupportedSources() ComponentSourceSupport {
	return ComponentSourceSupport{
		Embedded: true,
		Files:    true,
	}
}

func (d *CloudFormationAdapter) ValidateAndLoadConfig(ctx context.Context, c *manifestcore.Component) (ComponentConfig, []events.Event, error) {
	return nil, make([]events.Event, 0), nil
}

func (d *CloudFormationAdapter) Apply(ctx context.Context, c *manifestcore.Component, t runners.Runner) error {

	// Implement CloudFormation stack creation logic here
	return nil
}

func (d *CloudFormationAdapter) Destroy(ctx context.Context, c *manifestcore.Component, t runners.Runner) error {
	// Implement CloudFormation stack deletion logic here
	return nil
}

func init() {
	Register("cloudformation", &CloudFormationAdapter{})
}
