package orchestration

import "orch.io/pkg/varresolvers"

func shellCommandResolver(resolvers ...varresolvers.Resolver) varresolvers.Resolver {
	// Omit EnvResolver for command strings so environment values are not eagerly
	// interpolated into shell text. Commands still receive env through the runner.
	return &varresolvers.ChainResolver{Resolvers: resolvers}
}
