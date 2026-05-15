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

func NewAdapterContext(ctx context.Context, id string, logger logging.DebugLogger, emitter events.Emitter) context.Context {
	return context.WithValue(ctx, AdapterContextKey, AdapterContext{
		envID:   id,
		logger:  logger,
		emitter: emitter,
	})
}

func AdapterContextFromContext(ctx context.Context) (AdapterContext, bool) {
	aCtx, ok := ctx.Value(AdapterContextKey).(AdapterContext)
	return aCtx, ok
}
