package request

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

const addressVar = "address"
const blockHeightQuery = "block_height"

type GetAccount struct {
	Address flow.Address
	Height  uint64
}

func (g *GetAccount) Build(r *Request) error {
	return g.Parse(
		r.GetVar(addressVar),
		r.GetQueryParam(blockHeightQuery),
	)
}

func (g *GetAccount) Parse(rawAddress string, rawHeight string) error {
	address, err := ParseAddress(rawAddress)
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

const keyVar = "keyID"

type GetAccountKey struct {
	Address flow.Address
	KeyID   uint64
}

func (g *GetAccountKey) Build(r *Request) error {
	return g.Parse(
		r.GetVar(addressVar),
		r.GetVar(keyVar),
	)
}

func (g *GetAccountKey) Parse(rawAddress string, rawKeyID string) error {
	address, err := ParseAddress(rawAddress)
	if err != nil {
		return err
	}

	keyID, err := util.ToUint64(rawKeyID)
	if err != nil {
		return fmt.Errorf("invalid key index: %w", err)
	}

	g.Address = address
	g.KeyID = keyID

	return nil
}
