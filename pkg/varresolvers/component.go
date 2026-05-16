package varresolvers

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
)

type ComponentResolver struct {
	outputs map[string]string // componentName -> outputName -> value
	mutex   sync.RWMutex      // thread-safe for concurrent component execution
}

func NewComponentResolver() *ComponentResolver {
	return &ComponentResolver{
		outputs: make(map[string]string),
	}
}

// Resolve an expression like "component.outputs.db_url"
func (r *ComponentResolver) Resolve(ctx context.Context, expr string) (string, error) {
	parts := strings.Split(expr, ".")
	if len(parts) != 3 || parts[1] != "outputs" {
		return "", fmt.Errorf("invalid component output reference: %q", expr)
	}
	compName, outputName := parts[0], parts[2]

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if comp, ok := r.outputs[expr]; ok {
		return comp, nil
	}

	for key := range r.outputs {
		if strings.HasPrefix(key, compName+".outputs.") {
			return "", fmt.Errorf("output %q not found for component %q", outputName, compName)
		}
	}

	return "", fmt.Errorf("component %q not yet executed", compName)
}

// RegisterComponentOutput stores freshly produced adapter outputs after apply.
// Only outputs declared by the component manifest are made available for
// interpolation.
func (r *ComponentResolver) RegisterComponentOutput(componentName string, keys []string, outputs map[string]string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for outputName, value := range outputs {
		if !slices.Contains(keys, outputName) {
			continue // skip outputs not defined in the manifest
		}
		key := componentName + ".outputs." + outputName
		r.outputs[key] = value
	}
}

// RegisterPersistedComponentOutput rehydrates outputs that were already filtered
// and persisted in state, such as when destroy hooks need interpolation.
func (r *ComponentResolver) RegisterPersistedComponentOutput(componentName string, outputs map[string]string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for outputName, value := range outputs {
		key := componentName + ".outputs." + outputName
		r.outputs[key] = value
	}
}
