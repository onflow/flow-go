package protocol_state

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	storerr "github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProtocolStateMutator(t *testing.T) {
	suite.Run(t, new(MutatorSuite))
}

type MutatorSuite struct {
	suite.Suite
	protocolStateDB *storagemock.ProtocolState

	mutator *Mutator
}

func (s *MutatorSuite) SetupTest() {
	s.protocolStateDB = storagemock.NewProtocolState(s.T())
	s.mutator = NewMutator(s.protocolStateDB)
}

// TestCreateUpdaterForUnknownBlock tests that CreateUpdater returns an error if the parent protocol state is not found.
func (s *MutatorSuite) TestCreateUpdaterForUnknownBlock() {
	candidate := unittest.BlockHeaderFixture()
	s.protocolStateDB.On("ByBlockID", candidate.ParentID).Return(nil, storerr.ErrNotFound)
	updater, err := s.mutator.CreateUpdater(candidate)
	require.ErrorIs(s.T(), err, storerr.ErrNotFound)
	require.Nil(s.T(), updater)
}

// TestMutatorHappyPathNoChanges tests that Mutator correctly indexes the protocol state when there are no changes.
func (s *MutatorSuite) TestMutatorHappyPathNoChanges() {
	candidate := unittest.BlockHeaderFixture()
	parentState := unittest.ProtocolStateFixture()
	s.protocolStateDB.On("ByBlockID", candidate.ParentID).Return(parentState, nil)
	updater, err := s.mutator.CreateUpdater(candidate)
	require.NoError(s.T(), err)

	s.protocolStateDB.On("Index", candidate.ID(), parentState.ID()).Return(func(tx *transaction.Tx) error { return nil })

	err = s.mutator.CommitProtocolState(updater)(&transaction.Tx{})
	require.NoError(s.T(), err)
}

// TestMutatorHappyPathHasChanges tests that Mutator correctly persists and indexes the protocol state when there are changes.
func (s *MutatorSuite) TestMutatorHappyPathHasChanges() {
	parentState := unittest.ProtocolStateFixture()
	candidate := unittest.BlockHeaderFixture(unittest.HeaderWithView(parentState.CurrentEpochSetup.FirstView))
	s.protocolStateDB.On("ByBlockID", candidate.ParentID).Return(parentState, nil)
	updater, err := s.mutator.CreateUpdater(candidate)
	require.NoError(s.T(), err)

	// update protocol state so it has some changes
	updater.SetInvalidStateTransitionAttempted()
	updatedState, updatedStateID, hasChanges := updater.Build()
	require.True(s.T(), hasChanges)

	s.protocolStateDB.On("StoreTx", updatedStateID, updatedState).Return(func(tx *transaction.Tx) error { return nil })
	s.protocolStateDB.On("Index", candidate.ID(), updatedStateID).Return(func(tx *transaction.Tx) error { return nil })

	err = s.mutator.CommitProtocolState(updater)(&transaction.Tx{})
	require.NoError(s.T(), err)
}
