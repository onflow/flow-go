package dkg

import (
	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/model/flow"
	mockmodule "github.com/onflow/flow-go/module/mock"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestBeaconKeyRecovery(t *testing.T) {
	suite.Run(t, new(BeaconKeyRecoverySuite))
}

type BeaconKeyRecoverySuite struct {
	suite.Suite
	head               *flow.Header
	local              *mockmodule.Local
	state              *mockprotocol.State
	epochProtocolState *mockprotocol.EpochProtocolState
	dkgState           *mockstorage.EpochRecoveryMyBeaconKey
	finalSnapshot      *mockprotocol.Snapshot
	nextEpoch          *mockprotocol.Epoch

	currentEpochPhase flow.EpochPhase
}

func (s *BeaconKeyRecoverySuite) SetupTest() {
	s.local = mockmodule.NewLocal(s.T())
	s.state = mockprotocol.NewState(s.T())
	s.dkgState = mockstorage.NewEpochRecoveryMyBeaconKey(s.T())
	s.epochProtocolState = mockprotocol.NewEpochProtocolState(s.T())
	s.finalSnapshot = mockprotocol.NewSnapshot(s.T())
	s.nextEpoch = mockprotocol.NewEpoch(s.T())

	s.head = unittest.BlockHeaderFixture()
	s.currentEpochPhase = flow.EpochPhaseCommitted

	s.epochProtocolState.On("Epoch").Return(uint64(0))
	s.epochProtocolState.On("EpochPhase").Return(func() flow.EpochPhase { return s.currentEpochPhase })
	s.nextEpoch.On("Counter").Return(uint64(1), nil)

	epochs := mockprotocol.NewEpochQuery(s.T())
	epochs.On("Next").Return(s.nextEpoch, nil)

	s.finalSnapshot.On("Head").Return(s.head, nil)
	s.finalSnapshot.On("EpochProtocolState").Return(s.epochProtocolState, nil)
	s.finalSnapshot.On("Epochs").Return(epochs)

	s.state.On("Final").Return(s.finalSnapshot)
}

// TestNewBeaconKeyRecovery_KeyAlreadyRecovered tests a scenario:
// - node is in epoch committed phase
// - node has a safe beacon key for the next epoch
// In case like this there is no need for recovery and we should exit early.
func (s *BeaconKeyRecoverySuite) TestNewBeaconKeyRecovery_KeyAlreadyRecovered() {
	s.dkgState.On("RetrieveMyBeaconPrivateKey", uint64(1)).Return(
		unittest.PrivateKeyFixture(crypto.ECDSAP256, unittest.DefaultSeedFixtureLength), true, nil)

	recovery, err := NewBeaconKeyRecovery(unittest.Logger(), s.local, s.state, s.dkgState)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), recovery)

	s.dkgState.AssertNumberOfCalls(s.T(), "OverwriteMyBeaconPrivateKey", 0)
}
