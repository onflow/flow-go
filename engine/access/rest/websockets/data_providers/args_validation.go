package data_providers

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/onflow/flow-go/engine/access/rest/common"
	httpmodels "github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/model/flow"
)

// ErrInvalidArgument is a sentinel error returned by argument validation helpers in this package.
var ErrInvalidArgument = errors.New("invalid argument")

// ensureAllowedFields verifies that the provided fields map does not contain keys
// outside the provided allowedFields set.
//
// Expected errors:
//   - [data_providers.ErrInvalidArgument]: An unexpected field is present.
func ensureAllowedFields(fields map[string]interface{}, allowedFields map[string]struct{}) error {
	// Ensure only allowed fields are present
	for key := range fields {
		if _, exists := allowedFields[key]; !exists {
			return fmt.Errorf("unexpected field: '%s': %w", key, ErrInvalidArgument)
		}
	}

	return nil
}

// extractArrayOfStrings extracts an array of strings from the arguments map under the given key.
// When required is true, the key must be present.
//
// Expected errors:
//   - [data_providers.ErrInvalidArgument]: The value is missing (when required) or is not an array of strings.
func extractArrayOfStrings(args models.Arguments, name string, required bool) ([]string, error) {
	raw, exists := args[name]
	if !exists {
		if required {
			return nil, fmt.Errorf("missing '%s' field: %w", name, ErrInvalidArgument)
		}
		return []string{}, nil
	}

	converted, err := common.ConvertInterfaceToArrayOfStrings(raw)
	if err != nil {
		return nil, fmt.Errorf("'%s' must be an array of strings: %w", name, errors.Join(err, ErrInvalidArgument))
	}

	return converted, nil
}

// extractHeartbeatInterval extracts the optional 'heartbeat_interval' argument as uint64,
// returning the provided default when absent.
//
// Expected errors:
//   - [data_providers.ErrInvalidArgument]: The value is not a string or cannot be parsed as uint64.
func extractHeartbeatInterval(
	args models.Arguments,
	defaultHeartbeatInterval uint64,
) (uint64, error) {
	heartbeatIntervalRaw, exists := args["heartbeat_interval"]
	if !exists {
		return defaultHeartbeatInterval, nil
	}

	heartbeatIntervalString, ok := heartbeatIntervalRaw.(string)
	if !ok {
		return 0, fmt.Errorf("'heartbeat_interval' must be a string: %w", ErrInvalidArgument)
	}

	heartbeatInterval, err := strconv.ParseUint(heartbeatIntervalString, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("'heartbeat_interval' must be convertible to uint64: %w", errors.Join(err, ErrInvalidArgument))
	}

	return heartbeatInterval, nil
}

// extractExecutionStateQueryFields parses a nested execution-state query object from the
// provided arguments map and returns its fields.
//
// Parameters:
//   - args: incoming WebSocket arguments (untyped map).
//   - name: the key under which the nested object is expected (usually "execution_state_query").
//   - required: if true, the field must be present; otherwise it is optional and defaults apply.
//
// Expected input shape (map[string]interface{}):
//
//	{
//	  "agreeing_executors_count": "<uint64>",            // optional; defaults to 0
//	  "required_executor_ids": ["<hex-id>", ...],       // optional; defaults to nil
//	  "include_executor_metadata": "<bool>"              // optional; defaults to false
//	}
//
// Expected errors:
//   - [data_providers.ErrInvalidArgument]: The field is missing when required, the nested value is not a map,
//     or one of the contained fields fails validation (number/boolean parsing, array of hex IDs, etc.).
func extractExecutionStateQueryFields(
	args models.Arguments,
	name string,
	required bool,
) (httpmodels.ExecutionStateQuery, error) {
	var out httpmodels.ExecutionStateQuery
	executionStateRaw, exists := args[name]
	if !exists {
		if required {
			return httpmodels.ExecutionStateQuery{},
				fmt.Errorf("missing 'execution_state_query' field: %w", ErrInvalidArgument)
		}
		return httpmodels.ExecutionStateQuery{}, nil
	}

	executionStateQuery, ok := executionStateRaw.(map[string]interface{})
	if !ok {
		return httpmodels.ExecutionStateQuery{},
			fmt.Errorf("'execution_state_query' must be a map: %w", ErrInvalidArgument)
	}

	agreeingExecutorsCountRaw, exists := executionStateQuery["agreeing_executors_count"]
	if !exists {
		out.AgreeingExecutorsCount = 0
	} else {
		str, ok := agreeingExecutorsCountRaw.(string)
		if !ok {
			return httpmodels.ExecutionStateQuery{},
				fmt.Errorf("'agreeing_executors_count' must be a number: %w", ErrInvalidArgument)
		}
		val, err := strconv.ParseUint(str, 10, 64)
		if err != nil {
			return httpmodels.ExecutionStateQuery{},
				fmt.Errorf("'agreeing_executors_count' must be a number: %w", errors.Join(err, ErrInvalidArgument))
		}
		out.AgreeingExecutorsCount = val
	}

	requiredExecutorIDsRaw, exists := executionStateQuery["required_executor_ids"]
	if !exists {
		out.RequiredExecutorIDs = nil
	} else {
		converted, err := common.ConvertInterfaceToArrayOfStrings(requiredExecutorIDsRaw)
		if err != nil {
			return httpmodels.ExecutionStateQuery{},
				fmt.Errorf("'required_executor_ids' must be an array of strings: %w", errors.Join(err, ErrInvalidArgument))
		}

		ids, err := flow.IdentifierListFromHex(converted)
		if err != nil {
			return httpmodels.ExecutionStateQuery{},
				fmt.Errorf("'required_executor_ids' must be an array of strings: %w", errors.Join(err, ErrInvalidArgument))
		}
		// flow.IdentifierList is a []flow.Identifier, compatible with the target field type.
		out.RequiredExecutorIDs = ids
	}

	includeExecutorMetadataRaw, exists := executionStateQuery["include_executor_metadata"]
	if !exists {
		out.IncludeExecutorMetadata = false
	} else {
		str, ok := includeExecutorMetadataRaw.(string)
		if !ok {
			return httpmodels.ExecutionStateQuery{},
				fmt.Errorf("'include_executor_metadata' must be a boolean: %w", ErrInvalidArgument)
		}
		val, err := strconv.ParseBool(str)
		if err != nil {
			return httpmodels.ExecutionStateQuery{},
				fmt.Errorf("'include_executor_metadata' must be a boolean: %w", errors.Join(err, ErrInvalidArgument))
		}
		out.IncludeExecutorMetadata = val
	}

	return out, nil
}
