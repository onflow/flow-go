package dkg

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	model "github.com/onflow/flow-go/model/messages"
)

// TEMPORARY: The functionality to allow starting up a node without a properly configured
// machine account is very much intended to be temporary.

// This is required to support the very first mainnet spork with the epoch smart contract.
// At the beginning of the spork, operators will not have been able to generate their machine account
// because the smart contracts to do so have not been deployed yet. Therefore, for the duration of the spork,
// we allow this config to be omitted. For all subsequent sporks, it will be required.
// Implemented by: https://github.com/dapperlabs/flow-go/issues/5585
// Will be reverted by: https://github.com/dapperlabs/flow-go/issues/5619

type MockClient struct {
	log zerolog.Logger
}

func NewMockClient(log zerolog.Logger) *MockClient {

	log = log.With().Str("component", "mock_dkg_contract_client").Logger()
	return &MockClient{log: log}
}

func (c *MockClient) Broadcast(msg model.BroadcastDKGMessage) error {
	c.log.Fatal().Msg("caution: missing machine account configuration, but machine account used (Broadcast)")
	return nil
}

func (c *MockClient) ReadBroadcast(fromIndex uint, referenceBlock flow.Identifier) ([]model.BroadcastDKGMessage, error) {
	c.log.Fatal().Msg("caution: missing machine account configuration, but machine account used (ReadBroadcast)")
	return nil, nil
}

func (c *MockClient) SubmitResult(groupPublicKey crypto.PublicKey, publicKeys []crypto.PublicKey) error {
	c.log.Fatal().Msg("caution: missing machine account configuration, but machine account used (SubmitResult)")
	return nil
}
