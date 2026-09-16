//go:build windows

package hostenv

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func readHostEnvironment() (map[string]string, string, error) {
	machine, machineErr := readRegistryEnvironment(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`)
	user, userErr := readRegistryEnvironment(registry.CURRENT_USER, `Environment`)
	if machineErr != nil && userErr != nil {
		return nil, "windows_registry_machine_user", errors.Join(machineErr, userErr)
	}
	result := make(map[string]string, len(machine)+len(user))
	for key, value := range machine {
		result[key] = value
	}
	machinePath, machineHasPath := lookupFold(machine, "Path")
	userPath, userHasPath := lookupFold(user, "Path")
	for key, value := range user {
		result[key] = value
	}
	if machineHasPath || userHasPath {
		parts := make([]string, 0, 2)
		if strings.TrimSpace(machinePath) != "" {
			parts = append(parts, machinePath)
		}
		if strings.TrimSpace(userPath) != "" {
			parts = append(parts, userPath)
		}
		deleteFold(result, "Path")
		result["Path"] = strings.Join(parts, ";")
	}
	return result, "windows_registry_machine_user", nil
}

func readRegistryEnvironment(root registry.Key, path string) (map[string]string, error) {
	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("read Windows environment registry %s: %w", path, err)
	}
	defer key.Close()
	names, err := key.ReadValueNames(0)
	if err != nil {
		return nil, fmt.Errorf("list Windows environment registry %s: %w", path, err)
	}
	result := make(map[string]string, len(names))
	for _, name := range names {
		value, _, valueErr := key.GetStringValue(name)
		if valueErr != nil {
			continue
		}
		result[name] = value
	}
	return result, nil
}

func lookupFold(values map[string]string, name string) (string, bool) {
	for key, value := range values {
		if strings.EqualFold(key, name) {
			return value, true
		}
	}
	return "", false
}

func deleteFold(values map[string]string, name string) {
	for key := range values {
		if strings.EqualFold(key, name) {
			delete(values, key)
		}
	}
}
