package upgrades

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/model/flow"
)

type ProtocolVersionUpgradeSuite struct {
	Suite
}

func (s *ProtocolVersionUpgradeSuite) TestProtocolStateVersionUpgradeServiceEvent() {

	ctx := context.Background()

	serviceAddress := sdk.Address(s.net.Root().Header.ChainID.Chain().ServiceAddress())
	env := templates.Environment{
		NodeVersionBeaconAddress: serviceAddress.String(),
	}

	newProtocolVersion := uint64(2) // TODO doesn't matter for now
	activeView := uint64(0)         // active immediately

	txResult := s.sendUpgradeProtocolVersionTx(ctx, env, newProtocolVersion, activeView)
	s.Require().NoError(txResult.Error)

	sealed := s.ReceiptState.WaitForReceiptFromAny(
		s.T(),
		flow.Identifier(txResult.BlockID))

	s.Require().Len(sealed.ExecutionResult.ServiceEvents, 1)
	s.Require().IsType(
		&flow.ProtocolStateVersionUpgrade{},
		sealed.ExecutionResult.ServiceEvents[0].Event)

}

func (s *ProtocolVersionUpgradeSuite) sendUpgradeProtocolVersionTx(
	ctx context.Context,
	env templates.Environment,
	newProtocolVersion, activeView uint64,
) *sdk.TransactionResult {
	serviceAddress := s.net.Root().Header.ChainID.Chain().ServiceAddress()

	script := templates.GenerateSetProtocolStateVersionScript(env)

	latestBlockId, err := s.AccessClient().GetLatestBlockID(ctx)
	s.Require().NoError(err)
	seq := s.AccessClient().GetSeqNumber()

	tx := sdk.NewTransaction().
		SetScript(script).
		SetReferenceBlockID(sdk.Identifier(latestBlockId)).
		SetProposalKey(sdk.Address(serviceAddress), 0, seq).
		SetPayer(sdk.Address(serviceAddress)).
		AddAuthorizer(sdk.Address(serviceAddress))

	err = tx.AddArgument(cadence.NewUInt64(newProtocolVersion))
	s.Require().NoError(err)
	err = tx.AddArgument(cadence.NewUInt64(activeView))
	s.Require().NoError(err)

	err = s.AccessClient().SignAndSendTransaction(ctx, tx)
	s.Require().NoError(err)

	txResult, err := s.AccessClient().WaitForSealed(ctx, tx.ID())
	s.Require().NoError(err)
	return txResult
}

func TestProtocolVersionUpgrade(t *testing.T) {
	suite.Run(t, new(ProtocolVersionUpgradeSuite))
}
