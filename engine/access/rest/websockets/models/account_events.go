package models

import (
	"github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/model/flow"
)

type AccountEvents map[string]models.Events

func (a *AccountEvents) Build(accountEvents map[string]flow.EventsList) {
	result := make(map[string]models.Events, len(accountEvents))

	for i, e := range accountEvents {
		var events models.Events
		events.Build(e)
		result[i] = events
	}

	*a = result
}
