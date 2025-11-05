package cmdutil

import (
	"fmt"
	"os"
	"strings"
)

// LoadEnvFile loads environment variables from a file.
//
// Supported formats:
//   - KEY=VALUE
//   - export KEY=VALUE
//   - KEY="VALUE with spaces"
//   - # comments
//
// Empty lines and comments (starting with #) are skipped.
// If the file doesn't exist, returns an empty map (not an error).
func LoadEnvFile(path string) (map[string]string, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Return empty map if file doesn't exist (not an error)
		return make(map[string]string), nil
	}

	// Read file contents
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read env file: %w", err)
	}

	envVars := make(map[string]string)
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Remove "export " prefix if present
		line = strings.TrimPrefix(line, "export ")
		line = strings.TrimSpace(line)

		// Split on first '=' sign
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format in env file at line %d: %s", lineNum+1, line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if len(value) >= 2 {
			if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
				(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
				value = value[1 : len(value)-1]
			}
		}

		envVars[key] = value
	}

	return envVars, nil
}
