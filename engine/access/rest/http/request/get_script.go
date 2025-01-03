package request

import (
	"fmt"
	"io"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/model/flow"
)

const blockIDQuery = "block_id"

type GetScript struct {
	BlockID     flow.Identifier
	BlockHeight uint64
	Script      Script
}

// GetScriptRequest extracts necessary variables from the provided request,
// builds a GetScript instance, and validates it.
//
// No errors are expected during normal operation.
func GetScriptRequest(r *common.Request) (GetScript, error) {
	var req GetScript
	err := req.Build(r)
	return req, err
}

func (g *GetScript) Build(r *common.Request) error {
	return g.Parse(
		r.GetQueryParam(blockHeightQuery),
		r.GetQueryParam(blockIDQuery),
		r.Body,
	)
}

func (g *GetScript) Parse(rawHeight string, rawID string, rawScript io.Reader) error {
	var height Height
	err := height.Parse(rawHeight)
	if err != nil {
		return err
	}
	g.BlockHeight = height.Flow()

	var id parser.ID
	err = id.Parse(rawID)
	if err != nil {
		return err
	}
	g.BlockID = id.Flow()

	var script Script
	err = script.Parse(rawScript)
	if err != nil {
		return err
	}
	g.Script = script

	// default to last sealed block
	if g.BlockHeight == EmptyHeight && g.BlockID == flow.ZeroID {
		g.BlockHeight = SealedHeight
	}

	if g.BlockID != flow.ZeroID && g.BlockHeight != EmptyHeight {
		return fmt.Errorf("can not provide both block ID and block height")
	}

	return nil
}
