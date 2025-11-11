package models

import (
	"strconv"

	"github.com/onflow/flow-go/engine/access/rest/common/models"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/state_stream"
)

// EventResponse is the response message for 'events' topic.
type EventResponse struct {
	models.BlockEvents        // Embed BlockEvents struct to reuse its fields
	MessageIndex       uint64 `json:"message_index"`
}

// NewEventResponse creates EventResponse instance.
func NewEventResponse(eventsResponse *state_stream.EventsResponse, index uint64) *EventResponse {
	return &EventResponse{
		BlockEvents: commonmodels.BlockEvents{
			BlockId:        eventsResponse.BlockID.String(),
			BlockHeight:    strconv.FormatUint(eventsResponse.Height, 10),
			BlockTimestamp: eventsResponse.BlockTimestamp,
			Events:         commonmodels.NewEvents(eventsResponse.Events),
		},
		MessageIndex: index,
	}
}
