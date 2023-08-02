package request

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

const keyIndexVar = "key_index"

type GetAccountKey struct {
	Address  flow.Address
	KeyIndex uint64
	Height   uint64
}

func (g *GetAccountKey) Build(r *Request) error {
	return g.Parse(
		r.GetVar(addressVar),
		r.GetVar(keyIndexVar),
		r.GetQueryParam(blockHeightQuery),
	)
}

func (g *GetAccountKey) Parse(
	rawAddress string,
	rawKeyIndex string,
	rawHeight string,
) error {
	address, err := ParseAddress(rawAddress)
	if err != nil {
		return err
	}

	keyIndex, err := util.ToUint64(rawKeyIndex)
	if err != nil {
		return fmt.Errorf("invalid key index: %w", err)
	}

	var height Height
	err = height.Parse(rawHeight)
	if err != nil {
		return err
	}

	g.Address = address
	g.KeyIndex = keyIndex
	g.Height = height.Flow()

	// default to last block
	if g.Height == EmptyHeight {
		g.Height = SealedHeight
	}

	return nil
}
