package request

import (
	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/model/flow"
)

type GetAccountKeys struct {
	Address flow.Address
	Height  uint64
}

// GetAccountKeysRequest extracts necessary variables and query parameters from the provided request,
// builds a GetAccountKeys instance, and validates it.
//
// No errors are expected during normal operation.
func GetAccountKeysRequest(r *common.Request) (GetAccountKeys, error) {
	var req GetAccountKeys
	err := req.Build(r)
	return req, err
}

func (g *GetAccountKeys) Build(r *common.Request) error {
	return g.Parse(
		r.GetVar(addressVar),
		r.GetQueryParam(blockHeightQuery),
		r.Chain,
	)
}

func (g *GetAccountKeys) Parse(
	rawAddress string,
	rawHeight string,
	chain flow.Chain,
) error {
	address, err := parser.ParseAddress(rawAddress, chain)
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
