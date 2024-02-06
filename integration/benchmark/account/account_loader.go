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
		privateKey crypto.PrivateKey,
		hashAlgo crypto.HashAlgorithm,
	) (*FlowAccount, error)
}

type ClientAccountLoader struct {
	ctx        context.Context
	flowClient access.Client
}

func NewClientAccountLoader(
	ctx context.Context,
	flowClient access.Client,
) *ClientAccountLoader {
	return &ClientAccountLoader{
		ctx:        ctx,
		flowClient: flowClient,
	}
}

func (c *ClientAccountLoader) Load(
	address flowsdk.Address,
	privateKey crypto.PrivateKey,
	hashAlgo crypto.HashAlgorithm,
) (*FlowAccount, error) {
	return LoadAccount(c.ctx, c.flowClient, address, privateKey, hashAlgo)
}

func ReloadAccount(c Loader, acc *FlowAccount) error {
	newAcc, err := c.Load(acc.Address, acc.PrivateKey, acc.HashAlgo)
	if err != nil {
		return err
	}
	acc.keys = newAcc.keys
	return nil
}

var _ Loader = (*ClientAccountLoader)(nil)
