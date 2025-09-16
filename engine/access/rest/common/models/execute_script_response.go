package models

import (
	"github.com/onflow/flow-go/model/access"
)

// TODO(Uliana): add godoc
func NewExecuteScriptResponse(
	value []byte,
	executorMetadata access.ExecutorMetadata,
	includeExecutorMetadata bool,
) *ExecuteScriptResponse {
	var meta *Metadata
	if includeExecutorMetadata {
		m := NewMetadata(executorMetadata)
		meta = &m
	}

	return &ExecuteScriptResponse{
		Value:    value,
		Metadata: meta,
	}
}
