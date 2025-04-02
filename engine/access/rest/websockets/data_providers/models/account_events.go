package models

import (
	"github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/model/flow"
)

// AccountEvents represents a mapping of account addresses to their associated events.
type AccountEvents map[string]models.Events

// NewAccountEvents creates account events by converting each flow.EventsList to the corresponding models.Events.
func NewAccountEvents(accountEvents map[string]flow.EventsList) AccountEvents {
	result := make(map[string]models.Events, len(accountEvents))

	for i, e := range accountEvents {
		var events models.Events
		events.Build(e)
		result[i] = events
	}

	return result
}
