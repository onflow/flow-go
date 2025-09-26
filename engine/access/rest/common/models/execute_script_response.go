package models

import (
	"github.com/onflow/flow-go/model/access"
)

// TODO(Uliana): add godoc
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
