package provider

import (
	"context"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/flow"
)

type EventProvider interface {
	Events(
		ctx context.Context,
		blocks []BlockMetadata,
		eventType flow.EventType,
		encodingVersion entities.EventEncodingVersion,
		executionState *entities.ExecutionStateQuery,
	) (Response, entities.ExecutorMetadata, error)
}

// BlockMetadata is used to capture information about requested blocks to avoid repeated blockID
// calculations and passing around full block headers.
type BlockMetadata struct {
	ID        flow.Identifier
	Height    uint64
	Timestamp time.Time
}

type Response struct {
	Events        []flow.BlockEvents
	MissingBlocks []BlockMetadata
}
