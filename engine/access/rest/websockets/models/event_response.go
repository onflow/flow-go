package models

import (
	"strconv"

	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
)

// Build creates EventResponse instance.
func (e *EventResponse) Build(eventsResponse *backend.EventsResponse, index uint64) {
	var events commonmodels.Events
	events.Build(eventsResponse.Events)

	e.BlockEvents = commonmodels.BlockEvents{
		BlockId:        eventsResponse.BlockID.String(),
		BlockHeight:    strconv.FormatUint(eventsResponse.Height, 10),
		BlockTimestamp: eventsResponse.BlockTimestamp,
		Events:         events,
	}

	e.MessageIndex = index
}
