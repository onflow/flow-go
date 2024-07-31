package request

import (
	"github.com/onflow/flow-go/model/flow"
)

type GetAccountBalance struct {
	Address flow.Address
	Height  uint64
}

func (g *GetAccountBalance) Build(r *Request) error {
	return g.Parse(
		r.GetVar(addressVar),
		r.GetQueryParam(blockHeightQuery),
		r.Chain,
	)
}

func (g *GetAccountBalance) Parse(
	rawAddress string,
	rawHeight string,
	chain flow.Chain,
) error {
	address, err := ParseAddress(rawAddress, chain)
	if err != nil {
		return err
	}

	var height Height
	err = height.Parse(rawHeight)
	if err != nil {
		return err
	}

	g.Address = address
	g.Height = height.Flow()

	// default to last block
	if g.Height == EmptyHeight {
		g.Height = SealedHeight
	}

	return nil
}
