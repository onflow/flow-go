package models

import (
	"github.com/onflow/flow-go/model/access"
)

func NewMetadata(metadata access.ExecutorMetadata) Metadata {
	meta := NewExecutorMetadata(metadata)
	return Metadata{
		ExecutorMetadata: &meta,
	}
}

func NewExecutorMetadata(metadata access.ExecutorMetadata) ExecutorMetadata {
	return ExecutorMetadata{
		ExecutionResultId: metadata.ExecutionResultID.String(),
		ExecutorIds:       metadata.ExecutorIDs.Strings(),
	}
}
