package models

import "github.com/onflow/flow-go/model/flow"

type ExecutionStateQuery struct {
	AgreeingExecutorsCount  uint64            `json:"agreeing_executors_count,omitempty"`
	RequiredExecutorIds     []flow.Identifier `json:"required_executor_ids,omitempty"`
	IncludeExecutorMetadata bool              `json:"include_executor_metadata,omitempty"`
}
