package request

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

const startBlockIdQuery = "start_block_id"
const eventTypesQuery = "event_types"

type SubscribeEvents struct {
	StartBlockID flow.Identifier
	StartHeight  uint64

	EventTypes []string
	//Uliana: TODO: add events filter - contracts, addresses
}

func (g *SubscribeEvents) Build(r *Request) error {
	return g.Parse(
		r.GetQueryParam(startBlockIdQuery),
		r.GetQueryParam(startHeightQuery),
		r.GetQueryParams(eventTypesQuery),
	)
}

func (g *SubscribeEvents) Parse(rawBlockID string, rawStart string, rawTypes []string) error {
	var height Height
	err := height.Parse(rawStart)
	if err != nil {
		return fmt.Errorf("invalid start height: %w", err)
	}
	g.StartHeight = height.Flow()

	var startBlockID ID
	err = startBlockID.Parse(rawBlockID)
	if err != nil {
		return err
	}
	g.StartBlockID = startBlockID.Flow()

	// if both height and one or both of start and end height are provided
	if len(startBlockID) > 0 && g.StartHeight != EmptyHeight {
		return fmt.Errorf("can only provide either block ID or start height range")
	}

	var eventTypes EventTypes
	err = eventTypes.Parse(rawTypes)
	if err != nil {
		return err
	}

	g.EventTypes = eventTypes.Flow()

	return nil
}
