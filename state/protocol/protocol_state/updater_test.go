package protocol_state

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestUpdaterSuite(t *testing.T) {
	suite.Run(t, new(UpdaterSuite))
}

type UpdaterSuite struct {
	suite.Suite

	parentProtocolState *flow.RichProtocolStateEntry
	parentBlock         *flow.Header
	candidateBlock      *flow.Header

	updater *Updater
}

func (s *UpdaterSuite) SetupTest() {
	s.parentProtocolState = unittest.ProtocolStateFixture()
	s.parentBlock = unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentProtocolState.CurrentEpochSetup.FirstView + 1))
	s.candidateBlock = unittest.BlockHeaderWithParentFixture(s.parentBlock)

	s.updater = newUpdater(s.candidateBlock, s.parentProtocolState)
}

// TestNewUpdater tests if the constructor correctly setups invariants for updater.
func (s *UpdaterSuite) TestNewUpdater() {
	require.NotSame(s.T(), s.updater.parentState, s.updater.state, "except to take deep copy of parent state")
	require.Nil(s.T(), s.updater.parentState.NextEpochProtocolState)
	require.Nil(s.T(), s.updater.state.NextEpochProtocolState)
}

// TestTransitionToNextEpoch tests a scenario where the updater processes first block from next epoch.
// It has to discard the parent state and build a new state with data from next epoch.
func (s *UpdaterSuite) TestTransitionToNextEpoch() {
	// update protocol state with next epoch information
	unittest.WithNextEpochProtocolState()(s.parentProtocolState)

	candidate := unittest.BlockHeaderFixture(
		unittest.HeaderWithView(s.parentProtocolState.CurrentEpochSetup.FinalView + 1))
	// since candidate block is from next epoch, updater should transition to next epoch
	updater := newUpdater(candidate, s.parentProtocolState)
	updatedState, _, _ := updater.Build()
	require.Equal(s.T(), updatedState.ID(), s.parentProtocolState.NextEpochProtocolState.ID(), "should transition into next epoch")
	require.Nil(s.T(), updatedState.NextEpochProtocolState, "next epoch protocol state should be nil")
}

// TestBuild tests if the updater returns correct protocol state.
func (s *UpdaterSuite) TestBuild() {
	updatedState, stateID, hasChanges := s.updater.Build()
	require.Equal(s.T(), stateID, s.parentProtocolState.ID(), "should return same protocol state")
	require.False(s.T(), hasChanges, "should not have changes")
	require.NotSame(s.T(), updatedState, s.updater.state, "should return a copy of protocol state")

	s.updater.SetInvalidStateTransitionAttempted()
	updatedState, stateID, hasChanges = s.updater.Build()
	require.NotEqual(s.T(), stateID, s.parentProtocolState.ID(), "should return same protocol state")
	require.True(s.T(), hasChanges, "should have changes")

}
