package adapters

import (
	"context"
	"path"

	"orch.io/pkg/events"
	"orch.io/pkg/logging"
)

const AdapterContextKey = "__adapter.context"

type AdapterContext struct {
	envID   string
	logger  logging.DebugLogger
	emitter events.Emitter
}

func (a AdapterContext) GetComponentWorkDirInOrchLocalWorkDir(c string) string {
	return path.Join(".orch", a.envID, c)
}

func (a AdapterContext) EnvID() string {
	return a.envID
}

func (a AdapterContext) BuildRunnerWorkDir(baseWorkDir, componentName string) string {
	return path.Join(baseWorkDir, "orch", a.envID, componentName)
}

func NewAdapterContext(id string, logger logging.DebugLogger, emitter events.Emitter) AdapterContext {
	return AdapterContext{
		envID:   id,
		logger:  logger,
		emitter: emitter,
	}
}

func WithAdapterContext(ctx context.Context, aCtx AdapterContext) context.Context {
	return context.WithValue(ctx, AdapterContextKey, aCtx)
}

func AdapterContextFromContext(ctx context.Context) (AdapterContext, bool) {
	aCtx, ok := ctx.Value(AdapterContextKey).(AdapterContext)
	return aCtx, ok
}
