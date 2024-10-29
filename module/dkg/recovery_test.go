package dkg

import (
	"errors"
	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/model/flow"
	mockmodule "github.com/onflow/flow-go/module/mock"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
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

	currentEpochCounter uint64
	nextEpochCounter    uint64
	currentEpochPhase   flow.EpochPhase
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
	s.currentEpochCounter = uint64(0)
	s.nextEpochCounter = uint64(1)

	s.local.On("NodeID").Return(unittest.IdentifierFixture()).Maybe()
	s.epochProtocolState.On("Epoch").Return(s.currentEpochCounter).Maybe()
	s.epochProtocolState.On("EpochPhase").Return(func() flow.EpochPhase { return s.currentEpochPhase }).Maybe()
	s.nextEpoch.On("Counter").Return(s.nextEpochCounter, nil).Maybe()

	epochs := mockprotocol.NewEpochQuery(s.T())
	epochs.On("Next").Return(s.nextEpoch, nil).Maybe()

	s.finalSnapshot.On("Head").Return(s.head, nil)
	s.finalSnapshot.On("EpochProtocolState").Return(s.epochProtocolState, nil).Maybe()
	s.finalSnapshot.On("Epochs").Return(epochs).Maybe()

	s.state.On("Final").Return(s.finalSnapshot)
}

// TestNewBeaconKeyRecovery_EpochIsNotCommitted tests a scenario:
// - node is not in epoch committed phase
// In a case like this there is no need to proceed since we don't have the next epoch available.
func (s *BeaconKeyRecoverySuite) TestNewBeaconKeyRecovery_EpochIsNotCommitted() {
	for _, phase := range []flow.EpochPhase{
		flow.EpochPhaseFallback,
		flow.EpochPhaseStaking,
		flow.EpochPhaseSetup,
	} {
		s.currentEpochPhase = phase
		recovery, err := NewBeaconKeyRecovery(unittest.Logger(), s.local, s.state, s.dkgState)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), recovery)
	}
	s.dkgState.AssertNumberOfCalls(s.T(), "OverwriteMyBeaconPrivateKey", 0)
}

// TestNewBeaconKeyRecovery_HeadException tests a scenario:
// - exception is thrown when trying to get the head of the final snapshot
// This is an unexpected error and should be propagated to the caller.
func (s *BeaconKeyRecoverySuite) TestNewBeaconKeyRecovery_HeadException() {
	exception := errors.New("exception")
	s.finalSnapshot.On("Head").Unset()
	s.finalSnapshot.On("Head").Return(nil, exception).Once()

	recovery, err := NewBeaconKeyRecovery(unittest.Logger(), s.local, s.state, s.dkgState)
	require.ErrorIs(s.T(), err, exception)
	require.Nil(s.T(), recovery)
}

// TestNewBeaconKeyRecovery_EpochProtocolStateException tests a scenario:
// - exception is thrown when trying to get the epoch protocol state of the final snapshot
// This is an unexpected error and should be propagated to the caller.
func (s *BeaconKeyRecoverySuite) TestNewBeaconKeyRecovery_EpochProtocolStateException() {
	exception := errors.New("exception")
	s.finalSnapshot.On("EpochProtocolState").Unset()
	s.finalSnapshot.On("EpochProtocolState").Return(nil, exception).Once()

	recovery, err := NewBeaconKeyRecovery(unittest.Logger(), s.local, s.state, s.dkgState)
	require.ErrorIs(s.T(), err, exception)
	require.Nil(s.T(), recovery)
}

// TestNewBeaconKeyRecovery_NextEpochCounterException tests a scenario:
// - node is in epoch committed phase
// - exception is thrown when trying to get counter of the next epoch
// This is an unexpected error and should be propagated to the caller.
func (s *BeaconKeyRecoverySuite) TestNewBeaconKeyRecovery_NextEpochCounterException() {
	exception := errors.New("exception")
	s.nextEpoch.On("Counter").Unset()
	s.nextEpoch.On("Counter").Return(uint64(0), exception).Once()

	recovery, err := NewBeaconKeyRecovery(unittest.Logger(), s.local, s.state, s.dkgState)
	require.ErrorIs(s.T(), err, exception)
	require.Nil(s.T(), recovery)
}

// TestNewBeaconKeyRecovery_NextEpochRetrieveMyBeaconPrivateKeyException tests a scenario:
// - node is in epoch committed phase
// - exception is thrown when trying to check if there is a safe beacon key for the next epoch
// This is an unexpected error and should be propagated to the caller.
func (s *BeaconKeyRecoverySuite) TestNewBeaconKeyRecovery_NextEpochRetrieveMyBeaconPrivateKeyException() {
	exception := errors.New("exception")
	s.dkgState.On("RetrieveMyBeaconPrivateKey", s.nextEpochCounter).Return(nil, false, exception).Once()

	recovery, err := NewBeaconKeyRecovery(unittest.Logger(), s.local, s.state, s.dkgState)
	require.ErrorIs(s.T(), err, exception)
	require.Nil(s.T(), recovery)
}

// TestNewBeaconKeyRecovery_KeyAlreadyRecovered tests a scenario:
// - node is in epoch committed phase
// - node has a safe beacon key for the next epoch
// In case like this there is no need for recovery and we should exit early.
func (s *BeaconKeyRecoverySuite) TestNewBeaconKeyRecovery_KeyAlreadyRecovered() {
	s.dkgState.On("RetrieveMyBeaconPrivateKey", s.nextEpochCounter).Return(
		unittest.PrivateKeyFixture(crypto.ECDSAP256, unittest.DefaultSeedFixtureLength), true, nil).Once()

	recovery, err := NewBeaconKeyRecovery(unittest.Logger(), s.local, s.state, s.dkgState)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), recovery)

	s.dkgState.AssertNumberOfCalls(s.T(), "OverwriteMyBeaconPrivateKey", 0)
}

// TestNewBeaconKeyRecovery_NoSafeMyBeaconPrivateKey tests a scenario:
// - node is in epoch committed phase
// - node doesn't have a safe beacon key for the next epoch
// - node doesn't have a safe beacon key for the current epoch
// We can't do much in this case since there is no key to recover.
func (s *BeaconKeyRecoverySuite) TestNewBeaconKeyRecovery_NoSafeMyBeaconPrivateKey() {
	s.Run("no-safe-key", func() {
		dkgState := mockstorage.NewEpochRecoveryMyBeaconKey(s.T())
		dkgState.On("RetrieveMyBeaconPrivateKey", s.nextEpochCounter).Return(
			nil, false, nil).Once()
		dkgState.On("RetrieveMyBeaconPrivateKey", s.currentEpochCounter).Return(
			nil, false, nil).Once()

		recovery, err := NewBeaconKeyRecovery(unittest.Logger(), s.local, s.state, dkgState)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), recovery)

		dkgState.AssertNumberOfCalls(s.T(), "OverwriteMyBeaconPrivateKey", 0)
	})
	s.Run("err-not-found", func() {
		dkgState := mockstorage.NewEpochRecoveryMyBeaconKey(s.T())
		dkgState.On("RetrieveMyBeaconPrivateKey", s.nextEpochCounter).Return(
			nil, false, nil).Once()
		dkgState.On("RetrieveMyBeaconPrivateKey", s.currentEpochCounter).Return(
			nil, false, storage.ErrNotFound).Once()

		recovery, err := NewBeaconKeyRecovery(unittest.Logger(), s.local, s.state, dkgState)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), recovery)

		dkgState.AssertNumberOfCalls(s.T(), "OverwriteMyBeaconPrivateKey", 0)
	})
	s.Run("exception", func() {
		exception := errors.New("exception")
		dkgState := mockstorage.NewEpochRecoveryMyBeaconKey(s.T())
		dkgState.On("RetrieveMyBeaconPrivateKey", s.nextEpochCounter).Return(
			nil, false, nil).Once()
		dkgState.On("RetrieveMyBeaconPrivateKey", s.currentEpochCounter).Return(
			nil, false, exception).Once()

		recovery, err := NewBeaconKeyRecovery(unittest.Logger(), s.local, s.state, dkgState)
		require.ErrorIs(s.T(), err, exception)
		require.Nil(s.T(), recovery)

		dkgState.AssertNumberOfCalls(s.T(), "OverwriteMyBeaconPrivateKey", 0)
	})
}

// TestNewBeaconKeyRecovery_RecoverKey tests a scenario:
// - node is in epoch committed phase
// - node doesn't have a safe beacon key for the next epoch
// - node has a safe beacon key for the current epoch
// - node is part of the DKG for the next epoch
// In case like this we need try recovering the key from the current epoch.
func (s *BeaconKeyRecoverySuite) TestNewBeaconKeyRecovery_RecoverKey() {
	performTest := func(dkgState *mockstorage.EpochRecoveryMyBeaconKey) {
		// have a safe key for the current epoch
		myBeaconKey := unittest.PrivateKeyFixture(crypto.ECDSAP256, unittest.DefaultSeedFixtureLength)
		dkgState.On("RetrieveMyBeaconPrivateKey", s.currentEpochCounter).Return(myBeaconKey, true, nil).Once()
		// node is part of the DKG for the next epoch
		dkg := mockprotocol.NewDKG(s.T())
		dkg.On("KeyShare", s.local.NodeID()).Return(myBeaconKey.PublicKey(), nil).Once()
		s.nextEpoch.On("DKG").Return(dkg, nil).Once()

		dkgState.On("OverwriteMyBeaconPrivateKey", s.nextEpochCounter, myBeaconKey).Return(nil).Once()

		recovery, err := NewBeaconKeyRecovery(unittest.Logger(), s.local, s.state, dkgState)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), recovery)

		dkgState.AssertNumberOfCalls(s.T(), "OverwriteMyBeaconPrivateKey", 1)
	}

	s.Run("err-not-found-for-key-next-epoch", func() {
		dkgState := mockstorage.NewEpochRecoveryMyBeaconKey(s.T())
		dkgState.On("RetrieveMyBeaconPrivateKey", s.nextEpochCounter).Return(nil, false, storage.ErrNotFound).Once()
		performTest(dkgState)
	})
	s.Run("key-for-next-epoch-is-not-safe", func() {
		dkgState := mockstorage.NewEpochRecoveryMyBeaconKey(s.T())
		dkgState.On("RetrieveMyBeaconPrivateKey", s.nextEpochCounter).Return(nil, false, nil).Once()
		performTest(dkgState)
	})

}
