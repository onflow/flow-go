package data_providers

import (
	"fmt"
	"strconv"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
)

func ensureAllowedFields(fields map[string]interface{}, allowedFields map[string]struct{}) error {
	// Ensure only allowed fields are present
	for key := range fields {
		if _, exists := allowedFields[key]; !exists {
			return fmt.Errorf("unexpected field: '%s'", key)
		}
	}

	return nil
}

func extractArrayOfStrings(args models.Arguments, name string, required bool) ([]string, error) {
	raw, exists := args[name]
	if !exists {
		if required {
			return nil, fmt.Errorf("missing '%s' field", name)
		}
		return []string{}, nil
	}

	converted, err := common.ConvertInterfaceToArrayOfStrings(raw)
	if err != nil {
		return nil, fmt.Errorf("'%s' must be an array of strings: %w", name, err)
	}

	return converted, nil
}

// extractHeartbeatInterval extracts 'heartbeat_interval' argument which is always optional
func extractHeartbeatInterval(args models.Arguments, defaultHeartbeatInterval uint64) (uint64, error) {
	heartbeatIntervalRaw, exists := args["heartbeat_interval"]
	if !exists {
		return defaultHeartbeatInterval, nil
	}

	heartbeatIntervalString, ok := heartbeatIntervalRaw.(string)
	if !ok {
		return 0, fmt.Errorf("'heartbeat_interval' must be a string")
	}

	heartbeatInterval, err := strconv.ParseUint(heartbeatIntervalString, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("'heartbeat_interval' must be convertible to uint64: %w", err)
	}

	return heartbeatInterval, nil
}
