package models

type ExecutionStateQuery struct {
	AgreeingExecutorsCount  uint64   `json:"agreeing_executors_count,omitempty"`
	RequiredExecutorIds     [][]byte `json:"required_executor_ids,omitempty"`
	IncludeExecutorMetadata bool     `json:"include_executor_metadata,omitempty"`
}
