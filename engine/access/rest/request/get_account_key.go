package request

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

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
