package models

import (
	"fmt"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/model/flow"
	"io"
)

const blockIDQuery = "block_id"

type GetScriptRequest struct {
	BlockID     flow.Identifier
	BlockHeight uint64
	Script      Script
}

func (g *GetScriptRequest) Build(r *rest.Request) error {
	err := g.Parse(
		r.GetQueryParam(blockHeightQuery),
		r.GetQueryParam(blockIDQuery),
		r.Body,
	)
	if err != nil {
		return err
	}

	if len(g.BlockID) == 0 && g.BlockHeight == 0 {
		return rest.NewBadRequestError(fmt.Errorf("either block ID or block height must be provided"))
	}
	if len(g.BlockID) > 0 && g.BlockHeight != 0 {
		return rest.NewBadRequestError(fmt.Errorf("can not provide both block ID and block height"))
	}

	return nil
}

func (g *GetScriptRequest) Parse(rawHeight string, rawID string, rawScript io.Reader) error {
	var height Height
	err := height.Parse(rawHeight)
	if err != nil {
		return err
	}

	var id ID
	err = id.Parse(rawID)
	if err != nil {
		return err
	}

	var script Script
	err = script.Parse(rawScript)
	if err != nil {
		return err
	}

	g.BlockHeight = uint64(height)
	g.BlockID = flow.Identifier(id)
	g.Script = script

	return nil
}
