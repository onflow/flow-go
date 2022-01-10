package models

import (
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/model/flow"
)

func (e *ExecutionResult) Build(exeResult *flow.ExecutionResult, link rest.LinkGenerator) error {
	self, err := rest.SelfLink(exeResult.ID(), link.ExecutionResultLink)
	if err != nil {
		return err
	}

	events := make([]Event, len(exeResult.ServiceEvents))
	for i, e := range exeResult.ServiceEvents {
		events[i] = Event{
			Type_: e.Type,
		}
	}

	e.Id = exeResult.ID().String()
	e.BlockId = exeResult.BlockID.String()
	e.Events = events
	e.Links = self

	return nil
}
