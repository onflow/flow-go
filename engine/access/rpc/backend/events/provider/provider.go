package provider

import (
	"context"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

// EventProvider defines a source of Flow events for a set of blocks.
// Implementations retrieve and optionally transform events for the given
// blocks and event type, returning the results along with executor metadata.
//
// The returned Response contains events per block and information about
// any blocks that could not be served by the provider.
// ExecutorMetadata identifies the execution result and the executors used.
type EventProvider interface {
	Events(
		ctx context.Context,
		blocks []BlockMetadata,
		eventType flow.EventType,
		encodingVersion entities.EventEncodingVersion,
		executionResultInfo *optimistic_sync.ExecutionResultInfo,
	) (Response, *access.ExecutorMetadata, error)
}

// BlockMetadata is used to capture information about requested blocks to avoid repeated blockID
// calculations and passing around full block headers.
type BlockMetadata struct {
	ID        flow.Identifier
	Height    uint64
	Timestamp time.Time
}

// Response captures the outcome of an Events query from an EventProvider.
// - Events contains the per-block event lists matching the request.
// - MissingBlocks lists blocks that could not be served by the provider.
//
// Implementations may populate either field depending on availability and
// validation of data for each requested block.
// Consumers should handle MissingBlocks by retrying or querying a different
// provider as appropriate for their workflow.
type Response struct {
	Events        []flow.BlockEvents
	MissingBlocks []BlockMetadata
}
