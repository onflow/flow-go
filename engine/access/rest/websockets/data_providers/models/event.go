package models

import (
	"strconv"

	"github.com/onflow/flow-go/engine/access/rest/common/models"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
)

// EventResponse is the response message for 'events' topic.
type EventResponse struct {
	models.BlockEvents        // Embed BlockEvents struct to reuse its fields
	MessageIndex       uint64 `json:"message_index"`
}

// NewEventResponse creates EventResponse instance.
func NewEventResponse(eventsResponse *backend.EventsResponse, index uint64) *EventResponse {
	var events commonmodels.Events
	events.Build(eventsResponse.Events)

	return &EventResponse{
		BlockEvents: commonmodels.BlockEvents{
			BlockId:        eventsResponse.BlockID.String(),
			BlockHeight:    strconv.FormatUint(eventsResponse.Height, 10),
			BlockTimestamp: eventsResponse.BlockTimestamp,
			Events:         events,
		},
		MessageIndex: index,
	}
}
