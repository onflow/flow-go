package upgrades

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/integration/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"
	"github.com/onflow/flow-go/utils/unittest"
)

type ProtocolVersionUpgradeSuite struct {
	Suite
}

func TestProtocolVersionUpgrade(t *testing.T) {
	// See https://github.com/onflow/flow-go/pull/5840/files#r1589483631
	// Must merge and pin https://github.com/onflow/flow-core-contracts/pull/419 to re-enable test
	unittest.SkipUnless(t, unittest.TEST_TODO, "skipped as it depends on VersionBeacon contract upgrade")
	suite.Run(t, new(ProtocolVersionUpgradeSuite))
}

func (suite *ProtocolVersionUpgradeSuite) SetupTest() {
	// Begin the test with a v0 kvstore, rather than the default v1.
	// This lets us test upgrading v0->v1
	protocolState, err := suite.net.BootstrapSnapshot.ProtocolState()
	require.NoError(suite.T(), err)
	threshold := protocolState.GetEpochCommitSafetyThreshold()
	suite.KVStoreFactory = func(epochStateID flow.Identifier) (protocol_state.KVStoreAPI, error) {
		return kvstore.NewKVStoreV0(threshold, epochStateID)
	}
	suite.Suite.SetupTest()
}

// TestProtocolStateVersionUpgradeServiceEvent tests the process of upgrading the protocol
// state version using a service event.
//  1. Validate that an invalid upgrade event is ignored.
//  2. Validate that a valid upgrade event is accepted and results in an upgrade.
//  3. Validate that a valid upgrade event to an unknown version is accepted and results in a halt.
func (s *ProtocolVersionUpgradeSuite) TestProtocolStateVersionUpgradeServiceEvent() {
	ctx := context.Background()

	serviceAddress := sdk.Address(s.net.Root().Header.ChainID.Chain().ServiceAddress())
	env := templates.Environment{
		NodeVersionBeaconAddress: serviceAddress.String(),
	}

	const ACTIVE_VIEW_DIFF = 20 // active view is 20 above execution block view
	const INITIAL_PROTOCOL_VERSION = uint64(0)
	const NEXT_PROTOCOL_VERSION = uint64(1)    // valid version to upgrade to
	const UNKNOWN_PROTOCOL_VERSION = uint64(2) // invalid version to upgrade to

	// sanity check: we should start with a v0 kvstore
	snapshot := s.LatestProtocolStateSnapshot()
	actualProtocolVersion := snapshot.Encodable().SealingSegment.LatestProtocolStateEntry().KVStore.Version
	assert.Equal(s.T(), INITIAL_PROTOCOL_VERSION, actualProtocolVersion, "should have v0 initially")

	// 1. Invalid upgrade event should be ignored
	newProtocolVersion := uint64(1)                                             // version is valid
	txResult := s.sendUpgradeProtocolVersionTx(ctx, env, newProtocolVersion, 1) // invalid activeView
	s.Require().NoError(txResult.Error)

	// ensure the service event was included in a block
	sealed := s.ReceiptState.WaitForReceiptFromAny(s.T(), flow.Identifier(txResult.BlockID))
	s.Require().Len(sealed.ExecutionResult.ServiceEvents, 1)
	s.Require().IsType(&flow.ProtocolStateVersionUpgrade{}, sealed.ExecutionResult.ServiceEvents[0].Event)

	executedInBlock, ok := s.BlockState.ByBlockID(flow.Identifier(txResult.BlockID))
	require.True(s.T(), ok)
	invalidUpgradeActiveView := executedInBlock.Header.View + 1 // because we use a too-short activeViewDiff of 1

	// after an invalid protocol version upgrade event, we should still have a v0 kvstore
	snapshot = s.AwaitSnapshotAtView(invalidUpgradeActiveView, time.Minute, 500*time.Millisecond)
	actualProtocolVersion = snapshot.Encodable().SealingSegment.LatestProtocolStateEntry().KVStore.Version
	require.Equal(s.T(), INITIAL_PROTOCOL_VERSION, actualProtocolVersion, "should have v0 still after invalid upgrade")

	// 2. Valid service event should cause a version upgrade
	txResult = s.sendUpgradeProtocolVersionTx(ctx, env, NEXT_PROTOCOL_VERSION, ACTIVE_VIEW_DIFF)
	s.Require().NoError(txResult.Error)

	_ = s.ReceiptState.WaitForReceiptFromAny(s.T(), flow.Identifier(txResult.BlockID))

	executedInBlock, ok = s.BlockState.ByBlockID(flow.Identifier(txResult.BlockID))
	require.True(s.T(), ok)
	v1ActiveView := executedInBlock.Header.View + ACTIVE_VIEW_DIFF

	// wait for the version to become active, then validate our kvstore has upgraded to v1
	snapshot = s.AwaitSnapshotAtView(v1ActiveView, time.Minute, 500*time.Millisecond)
	actualProtocolVersion = snapshot.Encodable().SealingSegment.LatestProtocolStateEntry().KVStore.Version
	require.Equal(s.T(), NEXT_PROTOCOL_VERSION, actualProtocolVersion, "should have v1 after upgrade")

	// 3. Upgrade to unknown version should halt progress
	txResult = s.sendUpgradeProtocolVersionTx(ctx, env, UNKNOWN_PROTOCOL_VERSION, ACTIVE_VIEW_DIFF)
	s.Require().NoError(txResult.Error)

	_ = s.ReceiptState.WaitForReceiptFromAny(s.T(), flow.Identifier(txResult.BlockID))

	executedInBlock, ok = s.BlockState.ByBlockID(flow.Identifier(txResult.BlockID))
	require.True(s.T(), ok)
	v2ActiveView := executedInBlock.Header.View + ACTIVE_VIEW_DIFF

	// once consensus reaches v2ActiveView, progress should halt
	s.BlockState.WaitForHalt(s.T(), 10*time.Second, 100*time.Millisecond, time.Minute)
	require.LessOrEqual(s.T(), s.BlockState.HighestProposedView(), v2ActiveView)
}

// sendUpgradeProtocolVersionTx sends a governance transaction to upgrade the protocol state version.
// This causes a corresponding flow.ProtocolStateVersionUpgrade service event to be emitted.
// For these tests we use a special transaction which chooses the activation view for the
// new version relative to the execution view, to remove a potential source of flakiness.
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
