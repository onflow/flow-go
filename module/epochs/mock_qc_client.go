package epochs

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// TEMPORARY: The functionality to allow starting up a node without a properly configured
// machine account is very much intended to be temporary.

// This is required to support the very first mainnet spork with the epoch smart contract.
// At the beginning of the spork, operators will not have been able to generate their machine account
// because the smart contracts to do so have not been deployed yet. Therefore, for the duration of the spork,
// we allow this config to be omitted. For all subsequent sporks, it will be required.
// Implemented by: https://github.com/dapperlabs/flow-go/issues/5585
// Will be reverted by: https://github.com/dapperlabs/flow-go/issues/5619

type MockQCContractClient struct {
	log zerolog.Logger
}

func NewMockQCContractClient(log zerolog.Logger) *MockQCContractClient {
	log = log.With().Str("component", "mock_qc_contract_client").Logger()
	return &MockQCContractClient{log: log}
}

func (c *MockQCContractClient) SubmitVote(ctx context.Context, vote *model.Vote) error {
	c.log.Fatal().Msg("caution: missing machine account configuration, but machine account used (SubmitVote)")
	return nil
}

func (c *MockQCContractClient) Voted(ctx context.Context) (bool, error) {
	c.log.Fatal().Msg("caution: missing machine account configuration, but machine account used (Voted)")
	return false, nil
}
