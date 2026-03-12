package logging

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

// ParseComponentLogLevels parses the --component-log-levels flag value into a map of
// component pattern to zerolog.Level. Component patterns are normalized to lowercase.
//
// Format: "component:level,prefix.*:level"
// Example: "hotstuff:debug,network.*:warn"
//
// Any error indicates the entry is invalid.
func ParseComponentLogLevels(s string) (map[string]zerolog.Level, error) {
	result := make(map[string]zerolog.Level)
	if s == "" {
		return result, nil
	}
	for entry := range strings.SplitSeq(s, ",") {
		if strings.Count(entry, ":") != 1 {
			return nil, fmt.Errorf("invalid component log level entry %q: expected format component:level", entry)
		}
		parts := strings.SplitN(entry, ":", 2)
		component := NormalizePattern(strings.TrimSpace(parts[0]))
		levelStr := strings.ToLower(strings.TrimSpace(parts[1]))
		if component == "" {
			return nil, fmt.Errorf("invalid component log level entry %q: component name must not be empty", entry)
		}
		if err := ValidatePattern(component); err != nil {
			return nil, fmt.Errorf("invalid component log level entry %q: %w", entry, err)
		}
		level, err := zerolog.ParseLevel(levelStr)
		if err != nil {
			return nil, fmt.Errorf("invalid log level %q for component %q: %w", levelStr, component, err)
		}
		result[component] = level
	}
	return result, nil
}
