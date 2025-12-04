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
	if metadata == nil {
		return nil
	}

	return &ExecutorMetadata{
		ExecutionResultId: metadata.ExecutionResultID.String(),
		ExecutorIds:       metadata.ExecutorIDs.Strings(),
	}
}
