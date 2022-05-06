package request

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

const heightQuery = "height"
const startHeightQuery = "start_height"
const endHeightQuery = "end_height"
const MaxAllowedBlockRequestHeightRange = 50
const idParam = "id"

type GetBlock struct {
	Heights      []uint64
	StartHeight  uint64
	EndHeight    uint64
	FinalHeight  bool
	SealedHeight bool
}

func (g *GetBlock) Build(r *Request) error {
	return g.Parse(
		r.GetQueryParams(heightQuery),
		r.GetQueryParam(startHeightQuery),
		r.GetQueryParam(endHeightQuery),
	)
}

func (g *GetBlock) HasHeights() bool {
	return len(g.Heights) > 0
}

func (g *GetBlock) Parse(rawHeights []string, rawStart string, rawEnd string) error {
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
	if len(g.Heights) > 0 && (g.StartHeight != EmptyHeight || g.EndHeight != EmptyHeight) {
		return fmt.Errorf("can only provide either heights or start and end height range")
	}

	// if neither height nor start and end height are provided
	if len(heights) == 0 && (g.StartHeight == EmptyHeight || g.EndHeight == EmptyHeight) {
		return fmt.Errorf("must provide either heights or start and end height range")
	}

	if g.StartHeight > g.EndHeight {
		return fmt.Errorf("start height must be less than or equal to end height")
	}
	// check if range exceeds maximum but only if end is not equal to special value which is not known yet
	if g.EndHeight-g.StartHeight >= MaxAllowedBlockRequestHeightRange && g.EndHeight != FinalHeight && g.EndHeight != SealedHeight {
		return fmt.Errorf("height range %d exceeds maximum allowed of %d", g.EndHeight-g.StartHeight, MaxAllowedBlockRequestHeightRange)
	}

	if len(heights) > MaxAllowedBlockRequestHeightRange {
		return fmt.Errorf("at most %d heights can be requested at a time", MaxAllowedBlockRequestHeightRange)
	}

	// check that if sealed or final are used they are provided as only value as mix and matching heights with sealed is not encouraged
	if len(heights) > 1 {
		for _, h := range heights {
			if h == Height(SealedHeight) || h == Height(FinalHeight) {
				return fmt.Errorf("can not provide '%s' or '%s' values with other height values", final, sealed)
			}
		}
	} else if len(heights) == 1 {
		// if we have special values for heights set the booleans
		g.FinalHeight = heights[0] == Height(FinalHeight)
		g.SealedHeight = heights[0] == Height(SealedHeight)
	}

	return nil
}

type GetBlockByIDs struct {
	IDs []flow.Identifier
}

func (g *GetBlockByIDs) Build(r *Request) error {
	return g.Parse(
		r.GetVars(idParam),
	)
}

func (g *GetBlockByIDs) Parse(rawIds []string) error {
	var ids IDs
	err := ids.Parse(rawIds)
	if err != nil {
		return err
	}
	g.IDs = ids.Flow()

	return nil
}

type GetBlockPayload struct {
	GetByIDRequest
}
