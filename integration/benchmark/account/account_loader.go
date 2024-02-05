package account

import (
	"context"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access"
	"github.com/onflow/flow-go-sdk/crypto"
)

type Loader interface {
	Load(
		address flowsdk.Address,
		signer crypto.Signer,
	) (*FlowAccount, error)
}

type ClientAccountLoader struct {
	ctx        context.Context
	flowClient access.Client
}

func (c *ClientAccountLoader) Load(address flowsdk.Address, signer crypto.Signer) (*FlowAccount, error) {
	return LoadAccount(c.ctx, c.flowClient, address, signer)
}

func ReloadAccount(c Loader, acc *FlowAccount) error {
	key, err := acc.GetKey()
	if err != nil {
		return err
	}
	key.Done()

	newAcc, err := c.Load(acc.Address, key.Signer)
	if err != nil {
		return err
	}
	acc.keys = newAcc.keys
	return nil
}

var _ Loader = (*ClientAccountLoader)(nil)
