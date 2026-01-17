package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/flow"
)

// EventsResponse represents the response containing events for a specific block.
type EventsResponse struct {
	BlockID        flow.Identifier
	Height         uint64
	Events         flow.EventsList
	BlockTimestamp time.Time
}

// LegacyEventsProvider retrieves events by block height. It can be configured to retrieve events from
// the events indexer(if available) or using a dedicated callback to query it from other sources.
//
// Legacy: this type is deprecated and will be removed in the future.
type LegacyEventsProvider struct{}

// GetAllEventsResponse returns a function that retrieves the event response for a given block height.
// Expected errors:
// - codes.NotFound: If block header for the specified block height is not found.
// - error: An error, if any, encountered during getting events from storage or execution data.
func (b *LegacyEventsProvider) GetAllEventsResponse(ctx context.Context, height uint64) (*EventsResponse, error) {
	return &EventsResponse{}, fmt.Errorf("deprecated and should not be used")
}
