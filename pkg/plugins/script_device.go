package plugins

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

const scriptScheme = "script://"

// ResolveScriptDevice checks if path starts with the "script://" prefix.
// If so, it executes the remainder as a command and returns the trimmed stdout
// as the resolved device path. For any other value it returns path unchanged.
func ResolveScriptDevice(path string) (string, error) {
	if !strings.HasPrefix(path, scriptScheme) {
		return path, nil
	}

	cmdStr := strings.TrimPrefix(path, scriptScheme)
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return "", fmt.Errorf("script:// prefix provided but no command specified")
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("script %q failed: %w\nstderr: %s", parts[0], err, errMsg)
		}
		return "", fmt.Errorf("script %q failed: %w", parts[0], err)
	}

	device := strings.TrimSpace(stdout.String())
	if device == "" {
		return "", fmt.Errorf("script %q produced empty output", parts[0])
	}

	return device, nil
}
