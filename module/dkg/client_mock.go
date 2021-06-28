package dkg

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	model "github.com/onflow/flow-go/model/messages"
)

type MockClient struct {
	log zerolog.Logger
}

func NewMockClient(log zerolog.Logger) *MockClient {

	log = log.With().Str("component", "mock_dkg_contract_client").Logger()
	return &MockClient{log: log}
}

func (c *MockClient) Broadcast(msg model.BroadcastDKGMessage) error {
	return nil
}

func (c *MockClient) ReadBroadcast(fromIndex uint, referenceBlock flow.Identifier) ([]model.BroadcastDKGMessage, error) {
	return nil, nil
}

func (c *MockClient) SubmitResult(groupPublicKey crypto.PublicKey, publicKeys []crypto.PublicKey) error {
	return nil
}
