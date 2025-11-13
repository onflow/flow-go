package models

import (
	"github.com/onflow/flow-go/model/access"
)

// NewExecuteScriptResponse creates a new ExecuteScriptResponse.
//
// It wraps the provided script execution result and, if requested,
// includes the corresponding executor metadata in the response.
func NewExecuteScriptResponse(
	value []byte,
	executorMetadata *access.ExecutorMetadata,
	includeExecutorMetadata bool,
) *ExecuteScriptResponse {
	var meta *Metadata
	if includeExecutorMetadata {
		meta = NewMetadata(executorMetadata)
	}

	return &ExecuteScriptResponse{
		Value:    string(value),
		Metadata: meta,
	}
}
