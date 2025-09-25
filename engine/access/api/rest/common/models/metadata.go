package models

import (
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

func NewMetadata(metadata access.ExecutorMetadata) *Metadata {
	meta := NewExecutorMetadata(metadata)
	return &Metadata{
		ExecutorMetadata: &meta,
	}
}

func (m *Metadata) IsEmpty() bool {
	if len(m.ExecutorMetadata.ExecutionResultId) != 0 {
		return false
	}

	if len(m.ExecutorMetadata.ExecutorIds) != 0 {
		return false
	}

	return true
}

func NewExecutorMetadata(metadata access.ExecutorMetadata) ExecutorMetadata {
	if metadata.ExecutionResultID == flow.ZeroID {
		return ExecutorMetadata{}
	}

	return ExecutorMetadata{
		ExecutionResultId: metadata.ExecutionResultID.String(),
		ExecutorIds:       metadata.ExecutorIDs.Strings(),
	}
}
