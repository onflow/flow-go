package request

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

const eventTypeQuery = "type"
const blockQuery = "block_ids"
const MaxEventRequestHeightRange = 250

type GetEvents struct {
	StartHeight uint64
	EndHeight   uint64
	Type        string
	BlockIDs    []flow.Identifier
}

func (g *GetEvents) Build(r *Request) error {
	return g.Parse(
		r.GetQueryParam(eventTypeQuery),
		r.GetQueryParam(startHeightQuery),
		r.GetQueryParam(endHeightQuery),
		r.GetQueryParams(blockQuery),
	)
}

func (g *GetEvents) Parse(rawType string, rawStart string, rawEnd string, rawBlockIDs []string) error {
	var height Height
	err := height.Parse(rawStart)
	if err != nil {
		return fmt.Errorf("invalid start height: %w", err)
	}
	g.StartHeight = height.Flow()
	err = height.Parse(rawEnd)
	if err != nil {
		return fmt.Errorf("invalid end height: %w", err)
	}
	g.EndHeight = height.Flow()

	var blockIDs IDs
	err = blockIDs.Parse(rawBlockIDs)
	if err != nil {
		return err
	}
	g.BlockIDs = blockIDs.Flow()

	// if both height and one or both of start and end height are provided
	if len(blockIDs) > 0 && (g.StartHeight != EmptyHeight || g.EndHeight != EmptyHeight) {
		return fmt.Errorf("can only provide either block IDs or start and end height range")
	}

	// if neither height nor start and end height are provided
	if len(blockIDs) == 0 && (g.StartHeight == EmptyHeight || g.EndHeight == EmptyHeight) {
		return fmt.Errorf("must provide either block IDs or start and end height range")
	}

	if rawType == "" {
		return fmt.Errorf("event type must be provided")
	}
	var eventType EventType
	err = eventType.Parse(rawType)
	if err != nil {
		return err
	}
	g.Type = eventType.Flow()

	// validate start end height option
	if g.StartHeight != EmptyHeight && g.EndHeight != EmptyHeight {
		if g.StartHeight > g.EndHeight {
			return fmt.Errorf("start height must be less than or equal to end height")
		}
		// check if range exceeds maximum but only if end is not equal to special value which is not known yet
		if g.EndHeight-g.StartHeight >= MaxEventRequestHeightRange && g.EndHeight != FinalHeight && g.EndHeight != SealedHeight {
			return fmt.Errorf("height range %d exceeds maximum allowed of %d", g.EndHeight-g.StartHeight, MaxEventRequestHeightRange)
		}
	}

	return nil
}
