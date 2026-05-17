package varresolvers

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	manifestcore "orch.io/pkg/manifest/core"
)

type ComponentResolver struct {
	outputs                        map[string]string // componentName.outputs.outputName -> value
	unavailableSensitiveOutputRefs map[string]struct{}
	mutex                          sync.RWMutex // thread-safe for concurrent component execution
}

func NewComponentResolver() *ComponentResolver {
	return &ComponentResolver{
		outputs:                        make(map[string]string),
		unavailableSensitiveOutputRefs: make(map[string]struct{}),
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

	if _, ok := r.unavailableSensitiveOutputRefs[expr]; ok {
		return "", fmt.Errorf(
			"sensitive output %q is unavailable because component %q was already applied and skipped; sensitive outputs are not persisted in state",
			expr,
			compName,
		)
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
func (r *ComponentResolver) RegisterComponentOutput(componentName string, declared []manifestcore.Output, outputs map[string]string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for outputName, value := range outputs {
		if !slices.ContainsFunc(declared, func(output manifestcore.Output) bool {
			return output.Name == outputName
		}) {
			continue // skip outputs not defined in the manifest
		}
		key := componentName + ".outputs." + outputName
		r.outputs[key] = value
		delete(r.unavailableSensitiveOutputRefs, key)
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
		delete(r.unavailableSensitiveOutputRefs, key)
	}
}

// RegisterUnavailableSensitiveOutputs records sensitive outputs that exist in
// the manifest but cannot be rehydrated from persisted state. This keeps normal
// skipped-component behavior lazy: a skipped component can still provide
// non-sensitive persisted outputs, but a later interpolation of a sensitive
// output receives an explicit error instead of a vague "not found".
func (r *ComponentResolver) RegisterUnavailableSensitiveOutputs(componentName string, declared []manifestcore.Output, persisted map[string]string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, output := range declared {
		if !output.Sensitive {
			continue
		}
		if _, ok := persisted[output.Name]; ok {
			continue
		}
		key := componentName + ".outputs." + output.Name
		if _, ok := r.outputs[key]; ok {
			continue
		}
		r.unavailableSensitiveOutputRefs[key] = struct{}{}
	}
}
