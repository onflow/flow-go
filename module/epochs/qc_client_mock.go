package epochs

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

type QCContractClientMock struct {
	log zerolog.Logger
}

func NewQCContractClientMock(log zerolog.Logger) *QCContractClientMock {

	log = log.With().Str("component", "qc_contract_client_mock").Logger()
	return &QCContractClientMock{log: log}
}

func (c *QCContractClientMock) SubmitVote(ctx context.Context, vote *model.Vote) error {
	c.log.Fatal().Msg("NodeMachineAccountInfo file was not configured properly")
	return nil
}

func (c *QCContractClientMock) Voted(ctx context.Context) (bool, error) {
	c.log.Fatal().Msg("NodeMachineAccountInfo file was not configured properly")
	return false, nil
}
