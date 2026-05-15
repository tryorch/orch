package utils

import "fmt"

// MapToEnvSlice takes multiple maps of environment variables and
// merges them into a single slice of strings in the format "KEY=VALUE".
// If there are duplicate keys across the maps, the value from the last
// map will take precedence.
func MapToEnvSlice(envs ...map[string]string) []string {
	envMap := make(map[string]string)
	for _, env := range envs {
		for k, v := range env {
			envMap[k] = v
		}
	}
	envSlice := make([]string, 0, len(envMap))
	for k, v := range envMap {
		envSlice = append(envSlice, k+"="+v)
	}
	return envSlice
}

func RunnerComponentPrefix(runner, component string) string {
	return fmt.Sprintf("[%s > %s] ", runner, component)
}
