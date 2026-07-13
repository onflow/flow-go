package dkg

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestVerifyBeaconKeyForEpoch(t *testing.T) {
	suite.Run(t, new(VerifyBeaconKeyForEpochSuite))
}

type VerifyBeaconKeyForEpochSuite struct {
	suite.Suite

	nodeID        flow.Identifier
	epochCounter  uint64
	state         *mockprotocol.State
	finalSnapshot *mockprotocol.Snapshot
	epochs        *mockprotocol.EpochQuery
	currentEpoch  *mockprotocol.CommittedEpoch
	dkg           *mockprotocol.DKG
	beaconKeys    *mockstorage.SafeBeaconKeys
}

func (s *VerifyBeaconKeyForEpochSuite) SetupTest() {
	s.nodeID = unittest.IdentifierFixture()
	s.epochCounter = uint64(1)

	s.state = mockprotocol.NewState(s.T())
	s.finalSnapshot = mockprotocol.NewSnapshot(s.T())
	s.epochs = mockprotocol.NewEpochQuery(s.T())
	s.currentEpoch = mockprotocol.NewCommittedEpoch(s.T())
	s.dkg = mockprotocol.NewDKG(s.T())
	s.beaconKeys = mockstorage.NewSafeBeaconKeys(s.T())

	s.state.On("Final").Return(s.finalSnapshot).Maybe()
	s.finalSnapshot.On("Epochs").Return(s.epochs).Maybe()
	s.epochs.On("Current").Return(s.currentEpoch, nil).Maybe()
	s.currentEpoch.On("Counter").Return(s.epochCounter).Maybe()
	s.currentEpoch.On("DKG").Return(s.dkg, nil).Maybe()
}

// TestHappyPath tests a scenario where:
// - node is a DKG participant
// - beacon key exists and is safe
// - beacon key matches expected public key
// Should return nil (success).
func (s *VerifyBeaconKeyForEpochSuite) TestHappyPath() {
	myBeaconKey := unittest.PrivateKeyFixture(crypto.BLSBLS12381)
	expectedPubKey := myBeaconKey.PublicKey()

	s.dkg.On("KeyShare", s.nodeID).Return(expectedPubKey, nil).Once()
	s.beaconKeys.On("RetrieveMyBeaconPrivateKey", s.epochCounter).Return(myBeaconKey, true, nil).Once()

	err := VerifyBeaconKeyForEpoch(unittest.Logger(), s.nodeID, s.state, s.beaconKeys, true)
	require.NoError(s.T(), err)
}

// TestRequireKeyPresentFalse tests a scenario where:
// - requireKeyPresent is false
// - node is a DKG participant
// - beacon key is not found in storage
// Should return nil (not fail) because requireKeyPresent is false.
func (s *VerifyBeaconKeyForEpochSuite) TestRequireKeyPresentFalse() {
	myBeaconKey := unittest.PrivateKeyFixture(crypto.BLSBLS12381)
	expectedPubKey := myBeaconKey.PublicKey()

	s.dkg.On("KeyShare", s.nodeID).Return(expectedPubKey, nil).Once()
	s.beaconKeys.On("RetrieveMyBeaconPrivateKey", s.epochCounter).Return(nil, false, storage.ErrNotFound).Once()

	err := VerifyBeaconKeyForEpoch(unittest.Logger(), s.nodeID, s.state, s.beaconKeys, false)
	require.NoError(s.T(), err)
}

// TestNodeNotDKGParticipant tests a scenario where:
// - node is not a DKG participant for the current epoch
// Should return nil (skip verification).
func (s *VerifyBeaconKeyForEpochSuite) TestNodeNotDKGParticipant() {
	s.dkg.On("KeyShare", s.nodeID).Return(nil, protocol.IdentityNotFoundError{NodeID: s.nodeID}).Once()

	err := VerifyBeaconKeyForEpoch(unittest.Logger(), s.nodeID, s.state, s.beaconKeys, true)
	require.NoError(s.T(), err)
}

// TestBeaconKeyNotFound tests a scenario where:
// - node is a DKG participant
// - beacon key is not found in storage
// Should return error.
func (s *VerifyBeaconKeyForEpochSuite) TestBeaconKeyNotFound() {
	myBeaconKey := unittest.PrivateKeyFixture(crypto.BLSBLS12381)
	expectedPubKey := myBeaconKey.PublicKey()

	s.dkg.On("KeyShare", s.nodeID).Return(expectedPubKey, nil).Once()
	s.beaconKeys.On("RetrieveMyBeaconPrivateKey", s.epochCounter).Return(nil, false, storage.ErrNotFound).Once()

	err := VerifyBeaconKeyForEpoch(unittest.Logger(), s.nodeID, s.state, s.beaconKeys, true)
	require.Error(s.T(), err)
	require.ErrorIs(s.T(), err, storage.ErrNotFound)
	require.Contains(s.T(), err.Error(), "could not retrieve beacon key")
}

// TestBeaconKeyUnsafe tests a scenario where:
// - node is a DKG participant
// - beacon key exists but is marked as unsafe
// Should return error.
func (s *VerifyBeaconKeyForEpochSuite) TestBeaconKeyUnsafe() {
	myBeaconKey := unittest.PrivateKeyFixture(crypto.BLSBLS12381)
	expectedPubKey := myBeaconKey.PublicKey()

	s.dkg.On("KeyShare", s.nodeID).Return(expectedPubKey, nil).Once()
	s.beaconKeys.On("RetrieveMyBeaconPrivateKey", s.epochCounter).Return(myBeaconKey, false, nil).Once()

	err := VerifyBeaconKeyForEpoch(unittest.Logger(), s.nodeID, s.state, s.beaconKeys, true)
	require.Error(s.T(), err)
	require.Contains(s.T(), err.Error(), "marked unsafe")
}

// TestBeaconKeyNil tests a scenario where:
// - node is a DKG participant
// - beacon key is safe but nil
// Should return error.
func (s *VerifyBeaconKeyForEpochSuite) TestBeaconKeyNil() {
	myBeaconKey := unittest.PrivateKeyFixture(crypto.BLSBLS12381)
	expectedPubKey := myBeaconKey.PublicKey()

	s.dkg.On("KeyShare", s.nodeID).Return(expectedPubKey, nil).Once()
	s.beaconKeys.On("RetrieveMyBeaconPrivateKey", s.epochCounter).Return(nil, true, nil).Once()

	err := VerifyBeaconKeyForEpoch(unittest.Logger(), s.nodeID, s.state, s.beaconKeys, true)
	require.Error(s.T(), err)
	require.Contains(s.T(), err.Error(), "is nil")
}

// TestPublicKeyMismatch tests a scenario where:
// - node is a DKG participant
// - beacon key exists and is safe
// - beacon key does NOT match expected public key
// Should return error.
func (s *VerifyBeaconKeyForEpochSuite) TestPublicKeyMismatch() {
	myBeaconKey := unittest.PrivateKeyFixture(crypto.BLSBLS12381)
	differentPubKey := unittest.PublicKeysFixture(1, crypto.BLSBLS12381)[0]

	s.dkg.On("KeyShare", s.nodeID).Return(differentPubKey, nil).Once()
	s.beaconKeys.On("RetrieveMyBeaconPrivateKey", s.epochCounter).Return(myBeaconKey, true, nil).Once()

	err := VerifyBeaconKeyForEpoch(unittest.Logger(), s.nodeID, s.state, s.beaconKeys, true)
	require.Error(s.T(), err)
	require.Contains(s.T(), err.Error(), "does not match expected public key")
}

// TestGetCurrentEpochError tests a scenario where:
// - error getting current epoch from protocol state
// Should return error.
func (s *VerifyBeaconKeyForEpochSuite) TestGetCurrentEpochError() {
	exception := errors.New("exception")

	// Create fresh mocks for this test to avoid conflicts with SetupTest
	state := mockprotocol.NewState(s.T())
	finalSnapshot := mockprotocol.NewSnapshot(s.T())
	epochs := mockprotocol.NewEpochQuery(s.T())
	beaconKeys := mockstorage.NewSafeBeaconKeys(s.T())

	state.On("Final").Return(finalSnapshot)
	finalSnapshot.On("Epochs").Return(epochs)
	epochs.On("Current").Return(nil, exception).Once()

	err := VerifyBeaconKeyForEpoch(unittest.Logger(), s.nodeID, state, beaconKeys, true)
	require.Error(s.T(), err)
	require.ErrorIs(s.T(), err, exception)
}

// TestGetDKGError tests a scenario where:
// - error getting DKG info from current epoch
// Should return error.
func (s *VerifyBeaconKeyForEpochSuite) TestGetDKGError() {
	exception := errors.New("exception")

	s.currentEpoch.On("DKG").Unset()
	s.currentEpoch.On("DKG").Return(nil, exception).Once()

	err := VerifyBeaconKeyForEpoch(unittest.Logger(), s.nodeID, s.state, s.beaconKeys, true)
	require.Error(s.T(), err)
	require.ErrorIs(s.T(), err, exception)
}

// TestGetKeyShareException tests a scenario where:
// - unexpected error getting key share (not IdentityNotFoundError)
// Should return error.
func (s *VerifyBeaconKeyForEpochSuite) TestGetKeyShareException() {
	exception := errors.New("exception")

	s.dkg.On("KeyShare", s.nodeID).Return(nil, exception).Once()

	err := VerifyBeaconKeyForEpoch(unittest.Logger(), s.nodeID, s.state, s.beaconKeys, true)
	require.Error(s.T(), err)
	require.ErrorIs(s.T(), err, exception)
}

// TestRetrieveKeyException tests a scenario where:
// - node is a DKG participant
// - unexpected error retrieving beacon key (not ErrNotFound)
// Should return error.
func (s *VerifyBeaconKeyForEpochSuite) TestRetrieveKeyException() {
	exception := errors.New("exception")
	myBeaconKey := unittest.PrivateKeyFixture(crypto.BLSBLS12381)
	expectedPubKey := myBeaconKey.PublicKey()

	s.dkg.On("KeyShare", s.nodeID).Return(expectedPubKey, nil).Once()
	s.beaconKeys.On("RetrieveMyBeaconPrivateKey", s.epochCounter).Return(nil, false, exception).Once()

	err := VerifyBeaconKeyForEpoch(unittest.Logger(), s.nodeID, s.state, s.beaconKeys, true)
	require.Error(s.T(), err)
	require.ErrorIs(s.T(), err, exception)
}
