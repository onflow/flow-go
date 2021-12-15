package models

import (
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/model/flow"
)

const addressVar = "address"
const blockHeightQuery = "block_height"

type GetAccountRequest struct {
	address flow.Address
	height  uint64
}

func (g *GetAccountRequest) Build(r *rest.Request) error {
	return g.Parse(
		r.GetVar(addressVar),
		r.GetQueryParam(blockHeightQuery),
	)
}

func (g *GetAccountRequest) Parse(rawAddress string, rawHeight string) error {
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

	g.address = address.Flow()
	g.height = height.Flow()

	return nil
}
