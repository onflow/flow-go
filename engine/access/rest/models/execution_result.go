package models

import (
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

func (e *ExecutionResult) Build(exeResult *flow.ExecutionResult, link LinkGenerator) error {
	self, err := SelfLink(exeResult.ID(), link.ExecutionResultLink)
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

	e.PreviousResultId = exeResult.PreviousResultID.String()

	chunks := make([]Chunk, len(exeResult.Chunks))

	for i, flowChunk := range exeResult.Chunks {
		var chunk Chunk
		err = chunk.Build(flowChunk)
		if err != nil {
			return err
		}
		chunks[i] = chunk
	}
	e.Chunks = chunks
	return nil
}

func (c *Chunk) Build(chunk *flow.Chunk) error {
	c.BlockId = chunk.BlockID.String()
	c.Index = util.FromUint64(chunk.Index)
	c.CollectionIndex = util.FromUint64(uint64(chunk.CollectionIndex))
	c.StartState = util.ToBase64(chunk.StartState[:])
	c.EndState = util.ToBase64(chunk.EndState[:])
	c.NumberOfTransactions = util.FromUint64(chunk.NumberOfTransactions)
	c.EventCollection = chunk.EventCollection.String()
	c.TotalComputationUsed = util.FromUint64(chunk.TotalComputationUsed)
}
