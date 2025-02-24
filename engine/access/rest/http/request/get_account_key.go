package request

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

const indexVar = "index"

type GetAccountKey struct {
	Address flow.Address
	Index   uint32
	Height  uint64
}

// GetAccountKeyRequest extracts necessary variables and query parameters from the provided request,
// builds a GetAccountKey instance, and validates it.
//
// No errors are expected during normal operation.
func GetAccountKeyRequest(r *common.Request) (GetAccountKey, error) {
	var req GetAccountKey
	err := req.Build(r)
	return req, err
}

func (g *GetAccountKey) Build(r *common.Request) error {
	return g.Parse(
		r.GetVar(addressVar),
		r.GetVar(indexVar),
		r.GetQueryParam(blockHeightQuery),
		r.Chain,
	)
}

func (g *GetAccountKey) Parse(
	rawAddress string,
	rawIndex string,
	rawHeight string,
	chain flow.Chain,
) error {
	address, err := parser.ParseAddress(rawAddress, chain)
	if err != nil {
		return err
	}

	index, err := util.ToUint32(rawIndex)
	if err != nil {
		return fmt.Errorf("invalid key index: %w", err)
	}

	var height Height
	err = height.Parse(rawHeight)
	if err != nil {
		return err
	}

	g.Address = address
	g.Index = index
	g.Height = height.Flow()

	// default to last block
	if g.Height == EmptyHeight {
		g.Height = SealedHeight
	}

	return nil
}
