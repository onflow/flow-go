package request

import (
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/model/flow"
)

const addressVar = "address"
const blockHeightQuery = "block_height"

type GetAccount struct {
	Address flow.Address
	Height  uint64
}

func (g *GetAccount) Build(r *rest.Request) error {
	return g.Parse(
		r.GetVar(addressVar),
		r.GetQueryParam(blockHeightQuery),
	)
}

func (g *GetAccount) Parse(rawAddress string, rawHeight string) error {
	var address Address
	err := address.Parse(rawAddress)
	if err != nil {
		return err
	}

	var height Height
	err = height.Parse(rawHeight)
	if err != nil {
		return err
	}

	g.Address = address.Flow()
	g.Height = height.Flow()

	return nil
}
