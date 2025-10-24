package convert

import (
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

// NewCriteria converts a protobuf message ExecutionStateQuery type to an access-layer Criteria type
func NewCriteria(query *entities.ExecutionStateQuery) optimistic_sync.Criteria {
	if query == nil {
		return optimistic_sync.Criteria{}
	}

	return optimistic_sync.Criteria{
		AgreeingExecutorsCount:  uint(query.AgreeingExecutorsCount),
		RequiredExecutors:       MessagesToIdentifiers(query.RequiredExecutorIds),
		ParentExecutionResultID: flow.ZeroID,
	}
}
