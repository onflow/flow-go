package upgrades

import (
	"context"
	"github.com/onflow/flow-go/integration/utils"
	"github.com/onflow/flow-go/module/metrics"
	protocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/storage/badger"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-core-contracts/lib/go/templates"
	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/model/flow"
)

type ProtocolVersionUpgradeSuite struct {
	Suite
}

// TestProtocolStateVersionUpgradeServiceEvent tests the process of upgrading the protocol
// state version using a service event.
// First, we validate that an invalid upgrade event is ignored.
// Second, we validate that a valid upgrade event is accepted and results in an upgrade.
// Third, we validate that a valid upgrade event to an unknown version is accepted and results in a halt.
//
// TODO: Because the protocol state version is not currently exposed via the Snapshot API,
// we tear down the network then validate expected protocol version changes against the database.
func (s *ProtocolVersionUpgradeSuite) TestProtocolStateVersionUpgradeServiceEvent() {
	ctx := context.Background()

	serviceAddress := sdk.Address(s.net.Root().Header.ChainID.Chain().ServiceAddress())
	env := templates.Environment{
		NodeVersionBeaconAddress: serviceAddress.String(),
	}

	const ACTIVE_VIEW_DIFF = 25        // active view is 25 above execution block view
	const NEXT_PROTOCOL_VERSION = 1    // valid version to upgrade to
	const UNKNOWN_PROTOCOL_VERSION = 2 // invalid version to upgrade to

	// 1 - invalid service event should be ignored
	newProtocolVersion := uint64(1)                                             // version is valid
	txResult := s.sendUpgradeProtocolVersionTx(ctx, env, newProtocolVersion, 1) // invalid activeView
	s.Require().NoError(txResult.Error)

	// ensure the service event was included in a block
	sealed := s.ReceiptState.WaitForReceiptFromAny(s.T(), flow.Identifier(txResult.BlockID))
	s.Require().Len(sealed.ExecutionResult.ServiceEvents, 1)
	s.Require().IsType(&flow.ProtocolStateVersionUpgrade{}, sealed.ExecutionResult.ServiceEvents[0].Event)

	executedInBlock, ok := s.BlockState.ByBlockID(flow.Identifier(txResult.BlockID))
	require.True(s.T(), ok)
	invalidUpgradeActiveView := executedInBlock.Header.View + ACTIVE_VIEW_DIFF

	// 2 - valid service event should cause a version upgrade
	txResult = s.sendUpgradeProtocolVersionTx(ctx, env, NEXT_PROTOCOL_VERSION, ACTIVE_VIEW_DIFF)
	s.Require().NoError(txResult.Error)

	sealed = s.ReceiptState.WaitForReceiptFromAny(s.T(), flow.Identifier(txResult.BlockID))

	executedInBlock, ok = s.BlockState.ByBlockID(flow.Identifier(txResult.BlockID))
	require.True(s.T(), ok)
	v1ActiveView := executedInBlock.Header.View + ACTIVE_VIEW_DIFF

	// wait for the version to become active before sending the next upgrade transaction
	s.BlockState.WaitForSealedView(s.T(), v1ActiveView)

	// 3 - upgrade to unknown version should halt progress
	txResult = s.sendUpgradeProtocolVersionTx(ctx, env, UNKNOWN_PROTOCOL_VERSION, ACTIVE_VIEW_DIFF)
	s.Require().NoError(txResult.Error)

	sealed = s.ReceiptState.WaitForReceiptFromAny(s.T(), flow.Identifier(txResult.BlockID))

	executedInBlock, ok = s.BlockState.ByBlockID(flow.Identifier(txResult.BlockID))
	require.True(s.T(), ok)
	v2ActiveView := executedInBlock.Header.View + ACTIVE_VIEW_DIFF

	// once consensus reaches v2ActiveView, progress should halt
	s.BlockState.WaitForHalt(s.T(), 10*time.Second, 100*time.Millisecond, 60*time.Second)
	require.LessOrEqual(s.T(), s.BlockState.HighestProposedView(), v2ActiveView)

	// Stop containers so we can validate the expected protocol state version changes
	// occurred against their database.
	// We use a non-consensus node because consensus nodes may hard-crash,
	// leaving the db in a bad state where it can't be opened without a truncate.
	err := s.net.StopContainerByName(ctx, "access_1")
	require.NoError(s.T(), err)
	sn1Container := s.net.ContainerByName("access_1")
	err = sn1Container.WaitForContainerStopped(5 * time.Second)
	require.NoError(s.T(), err)

	state, err := sn1Container.OpenState()
	require.NoError(s.T(), err)
	db, err := sn1Container.DB()
	require.NoError(s.T(), err)
	kvStoreDB := badger.NewProtocolKVStore(metrics.NewNoopCollector(), db, 100, 100)

	// After the invalid upgrade and before the valid upgrade to v1, the protocol state version should remain on v0.
	blockBetweenInvalidUpgradeAndV1Upgrade := s.findBlockWithViewBetween(state, invalidUpgradeActiveView, v1ActiveView)
	v0KVStore, err := kvStoreDB.ByBlockID(blockBetweenInvalidUpgradeAndV1Upgrade.ID())
	require.NoError(s.T(), err)
	require.Equal(s.T(), uint64(0), v0KVStore.Version)

	// After v1 upgrade the protocol state version should be v1.
	blockBetweenV1UpgradeAndHalt := s.findBlockWithViewBetween(state, v1ActiveView, v2ActiveView)
	v1KVStore, err := kvStoreDB.ByBlockID(blockBetweenV1UpgradeAndHalt.ID())
	require.NoError(s.T(), err)
	require.Equal(s.T(), uint64(1), v1KVStore.Version)
}

// findBlockWithViewBetween doesn't try very hard to find a block with view between v1, v2 (exclusive).
// It relies on the property that, in integration tests, all blocks have the same or very close height and view.
// In this case this property does not hold, we fail the test.
func (s *ProtocolVersionUpgradeSuite) findBlockWithViewBetween(state *protocol.State, v1, v2 uint64) *flow.Header {
	head, err := state.AtHeight((v1 + v2) / 2).Head()
	require.NoError(s.T(), err)

	if head.View > v1 && head.View < v2 {
		return head
	}
	s.T().Fatalf("could not find block with view between %d-%d", v1, v2)
	return nil
}

func (s *ProtocolVersionUpgradeSuite) sendUpgradeProtocolVersionTx(
	ctx context.Context,
	env templates.Environment,
	newProtocolVersion, activeViewDiff uint64,
) *sdk.TransactionResult {
	latestBlockID, err := s.AccessClient().GetLatestBlockID(ctx)
	require.NoError(s.T(), err)

	tx, err := utils.MakeSetProtocolStateVersionTx(
		env,
		s.AccessClient().Account(),
		0,
		sdk.Identifier(latestBlockID),
		newProtocolVersion,
		activeViewDiff,
	)
	require.NoError(s.T(), err)

	err = s.AccessClient().SignAndSendTransaction(ctx, tx)
	require.NoError(s.T(), err)

	result, err := s.AccessClient().WaitForSealed(ctx, tx.ID())
	require.NoError(s.T(), err)
	s.AccessClient().Account().Keys[0].SequenceNumber++

	return result
}

func TestProtocolVersionUpgrade(t *testing.T) {
	suite.Run(t, new(ProtocolVersionUpgradeSuite))
}
