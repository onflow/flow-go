package convert

import (
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

func NewCriteria(query *entities.ExecutionStateQuery) optimistic_sync.Criteria {
	if query == nil {
		return optimistic_sync.Criteria{}
	}

	return optimistic_sync.Criteria{
		AgreeingExecutorsCount: uint(query.AgreeingExecutorsCount),
		RequiredExecutors:      MessagesToIdentifiers(query.RequiredExecutorIds),
	}
}
