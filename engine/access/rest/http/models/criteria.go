package models

import (
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

func NewCriteria(query ExecutionStateQuery) optimistic_sync.Criteria {
	return optimistic_sync.Criteria{
		AgreeingExecutorsCount: uint(query.AgreeingExecutorsCount),
		RequiredExecutors:      query.RequiredExecutorIDs,
	}
}
