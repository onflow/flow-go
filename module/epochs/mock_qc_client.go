package epochs

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

type MockQCContractClient struct {
	log zerolog.Logger
}

func NewMockQCContractClient(log zerolog.Logger) *MockQCContractClient {

	log = log.With().Str("component", "qc_contract_client_mock").Logger()
	return &MockQCContractClient{log: log}
}

func (c *MockQCContractClient) SubmitVote(ctx context.Context, vote *model.Vote) error {
	c.log.Fatal().Msg("NodeMachineAccountInfo file was not configured properly")
	return nil
}

func (c *MockQCContractClient) Voted(ctx context.Context) (bool, error) {
	c.log.Fatal().Msg("NodeMachineAccountInfo file was not configured properly")
	return false, nil
}
