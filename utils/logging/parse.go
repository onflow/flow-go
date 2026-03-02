package logging

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

// ParseComponentLogLevels parses the --component-log-levels flag value into a map of
// component pattern to zerolog.Level.
//
// Format: "component:level,prefix.*:level"
// Example: "hotstuff:debug,network.*:warn"
//
// Expected error returns during normal operation:
//   - error: if any entry is malformed or contains an unrecognized level string.
func ParseComponentLogLevels(s string) (map[string]zerolog.Level, error) {
	result := make(map[string]zerolog.Level)
	if s == "" {
		return result, nil
	}
	for _, entry := range strings.Split(s, ",") {
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid component log level entry %q: expected format component:level", entry)
		}
		component := strings.TrimSpace(parts[0])
		levelStr := strings.TrimSpace(parts[1])
		if component == "" {
			return nil, fmt.Errorf("invalid component log level entry %q: component name must not be empty", entry)
		}
		level, err := zerolog.ParseLevel(strings.ToLower(levelStr))
		if err != nil {
			return nil, fmt.Errorf("invalid log level %q for component %q: %w", levelStr, component, err)
		}
		result[component] = level
	}
	return result, nil
}
