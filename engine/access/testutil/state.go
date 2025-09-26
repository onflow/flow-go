package testutil

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
)

type MockState struct {
	State  *protocolmock.State
	Params *protocolmock.Params

	Blocks []*flow.Block

	Sealed    *flow.Block
	Finalized *flow.Block

	SealedSnapshot    *protocolmock.Snapshot
	FinalizedSnapshot *protocolmock.Snapshot
}

// NewMockState creates a new mock state with the given blocks.
// Blocks must have exactly 10 blocks, and blocks must form a chain.
// Block at index 2 is the latest sealed block.
// Block at index 6 is the latest finalized block.
// generate blocks using unittest.ChainBlockFixtureWithRoot
// e.g.
//
//	blocks := unittest.ChainBlockFixtureWithRoot(unittest.BlockHeaderFixture(), 10)
func NewMockState(t testing.TB, blocks []*flow.Block) *MockState {
	require.Len(t, blocks, 10)

	s := &MockState{
		State:             protocolmock.NewState(t),
		Params:            protocolmock.NewParams(t),
		SealedSnapshot:    protocolmock.NewSnapshot(t),
		FinalizedSnapshot: protocolmock.NewSnapshot(t),
		Blocks:            blocks,
	}

	s.Sealed = s.Blocks[2]
	s.Finalized = s.Blocks[6]

	// it's generally OK to ignore expectations on these because there will always be a subsequent
	// call on the returned snapshot which will have additional expectations.
	s.State.On("Sealed").Return(s.SealedSnapshot).Maybe()
	s.State.On("Final").Return(s.FinalizedSnapshot).Maybe()
	s.State.On("Params").Return(s.Params).Maybe()

	return s
}

// AllowValidBlocks configures the state to return snapshots for Sealed, Final, and AtBlockID using
// the configured blocks, as well as the Head method for each snapshot.
//
// While less precise than explicitly configuring expectations for each block, this makes it easier
// to perform blockbox testing that allows any valid data to be accessed
//
// Note: AtBlockID is only configured for the blocks in the state. Calls for any other blocks will
// cause the test to fail with an unexpected call error.
func (s *MockState) AllowValidBlocks(t testing.TB) *MockState {
	s.FinalizedSnapshot.On("Head").Return(s.Finalized.ToHeader(), nil).Maybe()
	s.SealedSnapshot.On("Head").Return(s.Sealed.ToHeader(), nil).Maybe()
	for _, block := range s.Blocks {
		snapshot := protocolmock.NewSnapshot(t)
		s.State.On("AtBlockID", block.ID()).Return(snapshot).Maybe()
		snapshot.On("Head").Return(block.ToHeader(), nil).Maybe()
	}
	return s
}

// AtBlockID configures AtBlockID to return a snapshot, and returns the snapshot.
func (s *MockState) AtBlockID(t testing.TB, blockID flow.Identifier) *protocolmock.Snapshot {
	snapshot := protocolmock.NewSnapshot(t)
	s.State.On("AtBlockID", blockID).Return(snapshot)
	return snapshot
}

// HeaderAt configures AtBlockID to return a snapshot who's Head method will return the given header.
func (s *MockState) HeaderAt(t testing.TB, header *flow.Header) {
	snapshot := protocolmock.NewSnapshot(t)
	s.State.On("AtBlockID", header.ID()).Return(snapshot)
	snapshot.On("Head").Return(header, nil)
}
