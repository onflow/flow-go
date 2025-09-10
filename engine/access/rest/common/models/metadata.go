package models

import (
	"github.com/onflow/flow-go/model/access"
)

func NewMetadata(metadata *access.ExecutorMetadata) *Metadata {
	if metadata == nil {
		return nil
	}

	return &Metadata{
		ExecutorMetadata: NewExecutorMetadata(metadata),
	}
}

func NewExecutorMetadata(metadata *access.ExecutorMetadata) *ExecutorMetadata {
	executorIDs := make([]string, len(metadata.ExecutorIDs))
	for i, id := range metadata.ExecutorIDs {
		executorIDs[i] = id.String()
	}

	return &ExecutorMetadata{
		ExecutionResultId: metadata.ExecutionResultID.String(),
		ExecutorIds:       executorIDs,
	}
}
