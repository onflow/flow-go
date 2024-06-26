package request

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

const indexVar = "index"

type GetAccountKey struct {
	Address flow.Address
	Index   uint64
	Height  uint64
}

func (g *GetAccountKey) Build(r *Request) error {
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
	address, err := ParseAddress(rawAddress, chain)
	if err != nil {
		return err
	}

	index, err := util.ToUint64(rawIndex)
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
