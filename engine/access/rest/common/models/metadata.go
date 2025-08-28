package models

import (
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

func NewMetadata(metadata entities.ExecutorMetadata) *Metadata {
	return &Metadata{
		ExecutorMetadata: NewExecutorMetadata(metadata),
	}
}

func NewExecutorMetadata(metadata entities.ExecutorMetadata) *ExecutorMetadata {
	executorIDs := make([]string, len(metadata.ExecutorId))
	for i, id := range metadata.ExecutorId {
		executorIDs[i] = string(id)
	}

	return &ExecutorMetadata{
		ExecutionResultId: string(metadata.ExecutionResultId),
		ExecutorIds:       executorIDs,
	}
}
