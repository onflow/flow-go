package state_stream

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

type AccountStatusesResponse struct {
	BlockID       flow.Identifier
	Height        uint64
	AccountEvents map[string]flow.EventsList
}

// EventsResponse represents the response containing events for a specific block.
type EventsResponse struct {
	BlockID        flow.Identifier
	Height         uint64
	Events         flow.EventsList
	BlockTimestamp time.Time
}

// ExecutionDataResponse bundles the execution data returned for a single block.
type ExecutionDataResponse struct {
	Height         uint64
	ExecutionData  *execution_data.BlockExecutionData
	BlockTimestamp time.Time
}
