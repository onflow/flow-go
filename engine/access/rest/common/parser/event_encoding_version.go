package parser

import (
	"fmt"
	"strings"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// ParseEventEncodingVersion parses the event_encoding_version REST query parameter.
// An empty string defaults to JSON_CDC_V0 to preserve prior behavior.
// Accepts the proto enum names ("JSON_CDC_V0", "CCF_V0") and their hyphenated
// aliases ("JSON-CDC-V0", "CCF-V0"), case-insensitively.
func ParseEventEncodingVersion(raw string) (entities.EventEncodingVersion, error) {
	if raw == "" {
		return entities.EventEncodingVersion_JSON_CDC_V0, nil
	}

	normalized := strings.ToUpper(strings.ReplaceAll(raw, "-", "_"))
	switch normalized {
	case "JSON_CDC_V0":
		return entities.EventEncodingVersion_JSON_CDC_V0, nil
	case "CCF_V0":
		return entities.EventEncodingVersion_CCF_V0, nil
	}
	return entities.EventEncodingVersion_JSON_CDC_V0,
		fmt.Errorf("invalid 'event_encoding_version', must be 'JSON-CDC-V0' or 'CCF-V0'")
}
