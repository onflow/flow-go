package models

import (
	"github.com/onflow/flow-go/model/flow"
)

func NewMetadata(metadata *flow.ExecutorMetadata) *Metadata {
	return &Metadata{
		ExecutorMetadata: NewExecutorMetadata(metadata),
	}
}

func NewExecutorMetadata(metadata *flow.ExecutorMetadata) *ExecutorMetadata {
	// metadata can be empty
	if metadata == nil {
		return &ExecutorMetadata{}
	}

	executorIDs := make([]string, len(metadata.ExecutorIDs))
	for i, id := range metadata.ExecutorIDs {
		executorIDs[i] = id.String()
	}

	return &ExecutorMetadata{
		ExecutionResultId: metadata.ExecutionResultID.String(),
		ExecutorIds:       executorIDs,
	}
}
