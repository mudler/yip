package plugins

import "fmt"

// ResolveScriptDevice checks if path starts with the "script://" prefix.
// If so, it executes the remainder as a command and returns the trimmed stdout
// as the resolved device path. For any other value it returns path unchanged.
func ResolveScriptDevice(path string) (string, error) {
	return "", fmt.Errorf("not implemented")
}
