package orchestration

import (
	"context"

	manifestcore "orch.io/pkg/manifest/core"
	"orch.io/pkg/varresolvers"
)

func componentExecutionEnv(envID string, component *manifestcore.Component, runnerName string, base map[string]string) map[string]string {
	env := make(map[string]string, len(base)+4)
	for key, value := range base {
		env[key] = value
	}

	env["ORCH_ENV_ID"] = envID
	env["ORCH_COMPONENT_NAME"] = component.Name
	env["ORCH_COMPONENT_TYPE"] = component.Type
	env["ORCH_RUNNER_NAME"] = runnerName

	return env
}

func interpolateEnv(ctx context.Context, env map[string]string, resolver varresolvers.Resolver) (map[string]string, error) {
	resolved := make(map[string]string, len(env))
	for key, value := range env {
		interpolated, err := varresolvers.InterpolateString(ctx, value, resolver)
		if err != nil {
			return nil, err
		}
		resolved[key] = interpolated
	}
	return resolved, nil
}
