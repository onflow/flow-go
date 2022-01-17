package request

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"regexp"
)

const eventTypeQuery = "type"
const blockQuery = "block_ids"

type GetEvents struct {
	StartHeight uint64
	EndHeight   uint64
	Type        string
	BlockIDs    []flow.Identifier
}

func (g *GetEvents) Build(r *Request) error {
	return g.Parse(
		r.GetQueryParam(startHeightQuery),
		r.GetQueryParam(endHeightQuery),
		r.GetQueryParam(eventTypeQuery),
		r.GetQueryParams(blockQuery),
	)
}

func (g *GetEvents) Parse(rawType string, rawStart string, rawEnd string, rawBlockIDs []string) error {
	var height Height
	err := height.Parse(rawStart)
	if err != nil {
		return err
	}
	g.StartHeight = height.Flow()
	err = height.Parse(rawEnd)
	if err != nil {
		return err
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

	g.Type = rawType
	if g.Type == "" {
		return fmt.Errorf("event type must be provided")
	}

	// match basic format A.address.contract.event (ignore err since regex will always compile)
	basic, _ := regexp.MatchString(`[A-Z]\.[a-f0-9]{16}\.[\w+]*\.[\w+]*`, g.Type)
	// match core events flow.event
	core, _ := regexp.MatchString(`flow\.[\w]*`, g.Type)

	if !core && !basic {
		return fmt.Errorf("invalid event type format")
	}

	// validate start end height option
	if g.StartHeight != EmptyHeight && g.EndHeight != EmptyHeight {
		if g.StartHeight > g.EndHeight {
			return fmt.Errorf("start height must be less than or equal to end height")
		}
		// check if range exceeds maximum but only if end is not equal to special value which is not known yet
		if g.EndHeight-g.StartHeight > MaxAllowedHeights && g.EndHeight != FinalHeight && g.EndHeight != SealedHeight {
			return fmt.Errorf("height range %d exceeds maximum allowed of %d", g.EndHeight-g.StartHeight, MaxAllowedHeights)
		}
	}

	return nil
}
