package orchestration

import (
	"fmt"
	"strings"

	manifestcore "orch.io/pkg/manifest/core"
)

// TopologicallySortComponents sorts the given components based on their dependencies
// and returns the planned operations in order.
func TopologicallySortComponents(
	elements []manifestcore.Component,
) ([]*manifestcore.Component, error) {

	nodes := make(map[string]*manifestcore.Component, len(elements))
	for _, e := range elements {
		if _, exists := nodes[e.Name]; exists {
			// very unlikely due to earlier validation, but just in case
			return nil, fmt.Errorf("duplicate executable key: %s", e.Name)
		}
		nodes[e.Name] = &e
	}

	// Validate dependencies exist
	for _, e := range elements {
		for _, dep := range e.DependsOn {
			if _, exists := nodes[dep]; !exists {
				return nil, fmt.Errorf(
					"component %q depends on unknown component %q",
					e.Name,
					dep,
				)
			}
		}
	}

	// 0 = unvisited, 1 = visiting, 2 = visited
	state := make(map[string]int)
	stack := make([]string, 0)
	indexInStack := make(map[string]int)

	result := make([]*manifestcore.Component, 0, len(elements))

	var visit func(string) error
	visit = func(key string) error {
		switch state[key] {
		case 1:
			// Cycle detected — reconstruct path
			start := indexInStack[key]
			cycle := append(stack[start:], key)
			return fmt.Errorf(
				"circular dependency detected: %s",
				strings.Join(cycle, " -> "),
			)
		case 2:
			return nil
		}

		state[key] = 1
		indexInStack[key] = len(stack)
		stack = append(stack, key)

		for _, dep := range nodes[key].DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}

		// Pop from stack
		stack = stack[:len(stack)-1]
		delete(indexInStack, key)
		state[key] = 2

		result = append(result, nodes[key])
		return nil
	}

	for _, e := range elements {
		if state[e.Name] == 0 {
			if err := visit(e.Name); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}
