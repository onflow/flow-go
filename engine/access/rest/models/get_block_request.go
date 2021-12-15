package models

import (
	"fmt"
	"github.com/onflow/flow-go/engine/access/rest"
)

const heightQuery = "height"
const startHeightQuery = "start_height"
const endHeightQuery = "end_height"
const maxAllowedHeights = 50

type GetBlockRequest struct {
	Heights      []uint64
	StartHeight  uint64
	EndHeight    uint64
	FinalHeight  bool
	SealedHeight bool
}

func (g *GetBlockRequest) Build(r *rest.Request) error {
	err := g.Parse(
		r.GetQueryParams(heightQuery),
		r.GetQueryParam(startHeightQuery),
		r.GetQueryParam(endHeightQuery),
	)

	if err != nil {
		return rest.NewBadRequestError(err)
	}

	return nil
}

func (g *GetBlockRequest) HasHeights() bool {
	return len(g.Heights) > 0
}

func (g *GetBlockRequest) Parse(rawHeights []string, rawStart string, rawEnd string) error {
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

	var heights Heights
	err = heights.Parse(rawHeights)
	if err != nil {
		return err
	}
	g.Heights = heights.Flow()

	// if both height and one or both of start and end height are provided
	if len(g.Heights) > 0 && (g.StartHeight != 0 || g.EndHeight != 0) {
		return fmt.Errorf("can only provide either heights or start and end height range")
	}

	// if neither height nor start and end height are provided
	if len(heights) == 0 && (g.StartHeight == 0 || g.EndHeight == 0) {
		return fmt.Errorf("must provide either heights or start and end height range")
	}

	if g.StartHeight > g.EndHeight {
		return fmt.Errorf("start height must be less than or equal to end height")
	}

	if g.EndHeight-g.StartHeight > maxAllowedHeights {
		return fmt.Errorf("height range %d exceeds maximum allowed of %d", g.EndHeight-g.StartHeight, maxAllowedHeights)
	}

	// check that if sealed or final are used they are provided as only value as mix and matching heights with sealed is not encouraged
	if len(heights) > 0 {
		for _, h := range heights {
			if h == SealedHeight || h == FinalHeight {
				return fmt.Errorf("can not provide '%s' or '%s' values with other height values", final, sealed)
			}
		}
	}

	g.FinalHeight = heights[0] == FinalHeight
	g.SealedHeight = heights[0] == SealedHeight

	return nil
}
