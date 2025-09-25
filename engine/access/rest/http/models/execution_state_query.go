package models

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

type ExecutionStateQuery struct {
	AgreeingExecutorsCount  uint64            `json:"agreeing_executors_count,omitempty"`
	RequiredExecutorIDs     []flow.Identifier `json:"required_executor_ids,omitempty"`
	IncludeExecutorMetadata bool              `json:"include_executor_metadata,omitempty"`
}

func NewCriteria(query ExecutionStateQuery) optimistic_sync.Criteria {
	return optimistic_sync.Criteria{
		AgreeingExecutorsCount: uint(query.AgreeingExecutorsCount),
		RequiredExecutors:      query.RequiredExecutorIDs,
	}
}
