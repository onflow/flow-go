package account

import (
	"context"
	"github.com/rs/zerolog"

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
	log        zerolog.Logger
	ctx        context.Context
	flowClient access.Client
}

func NewClientAccountLoader(
	log zerolog.Logger,
	ctx context.Context,
	flowClient access.Client,
) *ClientAccountLoader {
	return &ClientAccountLoader{
		log:        log.With().Str("component", "account_loader").Logger(),
		ctx:        ctx,
		flowClient: flowClient,
	}
}

func (c *ClientAccountLoader) Load(
	address flowsdk.Address,
	privateKey crypto.PrivateKey,
	hashAlgo crypto.HashAlgorithm,
) (*FlowAccount, error) {
	acc, err := LoadAccount(c.ctx, c.flowClient, address, privateKey, hashAlgo)

	c.log.Debug().
		Str("address", address.String()).
		Int("keys", acc.NumKeys()).
		Msg("Loaded account")

	return acc, err
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
