package models

import (
	"github.com/onflow/flow-go/model/flow"
)

func NewMetadata(metadata *flow.ExecutorMetadata) *Metadata {
	meta := NewExecutorMetadata(metadata)
	// if meta is nil, we don't want to allocate memory for an object
	// so that it can be omitted in during conversion to JSON
	if meta == nil {
		return nil
	}

	return &Metadata{
		ExecutorMetadata: NewExecutorMetadata(metadata),
	}
}

func NewExecutorMetadata(metadata *flow.ExecutorMetadata) *ExecutorMetadata {
	// metadata can be empty
	if metadata == nil {
		return &ExecutorMetadata{}
	}

	// we don't want to allocate memory for an object if it is empty
	// so that it can be omitted in during conversion to JSON
	if metadata.ExecutionResultID == flow.ZeroID &&
		len(metadata.ExecutorIDs) == 0 {
		return nil
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
