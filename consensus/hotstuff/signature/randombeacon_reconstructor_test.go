package signature

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	mockhotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestRandomBeaconReconstructor_InvalidSignerID tests that RandomBeaconReconstructor doesn't forward calls to
// RandomBeaconInspector if it fails to map signerID to signerIndex
func TestRandomBeaconReconstructor_InvalidSignerID(t *testing.T) {
	committee := unittest.IdentityListFixture(5)
	dkg := &mockhotstuff.DKG{}
	dkg.On("Size").Return(uint(len(committee)))
	dkg.On("GroupKey").Return(helper.MakeBLSKey(t).PublicKey())
	for _, identity := range committee {
		keyShare := helper.MakeBLSKey(t).PublicKey()
		dkg.On("KeyShare", identity.NodeID).Return(keyShare, nil).Once()
	}

	reconstructor, err := NewRandomBeaconReconstructor(committee, unittest.RandomBytes(32), dkg)
	require.NoError(t, err)
	exception := errors.New("invalid-signer-id")
	t.Run("trustedAdd", func(t *testing.T) {
		signerID := unittest.IdentifierFixture()
		dkg.On("Index", signerID).Return(uint(0), exception).Once()
		_, err := reconstructor.TrustedAdd(signerID, unittest.SignatureFixture())
		require.ErrorAs(t, err, &exception)
	})
	t.Run("verify", func(t *testing.T) {
		signerID := unittest.IdentifierFixture()
		dkg.On("Index", signerID).Return(uint(0), exception).Once()
		err := reconstructor.Verify(signerID, unittest.SignatureFixture())
		require.ErrorAs(t, err, &exception)
	})

	dkg.AssertExpectations(t)
}

// TestNewRandomBeaconReconstructor tests construction process for RandomBeaconReconstructor
// All errors need to be correctly handled
func TestNewRandomBeaconReconstructor(t *testing.T) {
	committee := unittest.IdentityListFixture(5)
	msg := unittest.RandomBytes(32)
	t.Run("no-key-share", func(t *testing.T) {
		dkg := &mockhotstuff.DKG{}
		dkg.On("KeyShare", mock.Anything).Return(nil, protocol.IdentityNotFoundError{NodeID: unittest.IdentifierFixture()})
		reconstructor, err := NewRandomBeaconReconstructor(committee, msg, dkg)
		require.True(t, protocol.IsIdentityNotFound(err))
		require.Nil(t, reconstructor)
	})
	t.Run("invalid-group-key", func(t *testing.T) {
		dkg := &mockhotstuff.DKG{}
		dkg.On("Size").Return(uint(len(committee)))
		for _, identity := range committee {
			keyShare := helper.MakeBLSKey(t).PublicKey()
			dkg.On("KeyShare", identity.NodeID).Return(keyShare, nil).Once()
		}
		// not bls key
		dkg.On("GroupKey").Return(unittest.NetworkingPrivKeyFixture().PublicKey()).Once()
		reconstructor, err := NewRandomBeaconReconstructor(committee, msg, dkg)
		require.True(t, engine.IsInvalidInputError(err))
		require.Nil(t, reconstructor)
	})
	t.Run("invalid-key-share", func(t *testing.T) {
		dkg := &mockhotstuff.DKG{}
		dkg.On("Size").Return(uint(len(committee)))
		for _, identity := range committee {
			// not bls key
			dkg.On("KeyShare", identity.NodeID).Return(identity.NetworkPubKey, nil).Once()
		}
		// valid group key
		dkg.On("GroupKey").Return(helper.MakeBLSKey(t).PublicKey()).Once()
		reconstructor, err := NewRandomBeaconReconstructor(committee, msg, dkg)
		require.True(t, engine.IsInvalidInputError(err))
		require.Nil(t, reconstructor)
	})
	t.Run("invalid-size", func(t *testing.T) {
		dkg := &mockhotstuff.DKG{}
		// invalid size
		dkg.On("Size").Return(uint(0))
		// valid shares
		for _, identity := range committee {
			keyShare := helper.MakeBLSKey(t).PublicKey()
			dkg.On("KeyShare", identity.NodeID).Return(keyShare, nil).Once()

		}
		// valid group key
		dkg.On("GroupKey").Return(helper.MakeBLSKey(t).PublicKey()).Once()
		reconstructor, err := NewRandomBeaconReconstructor(committee, msg, dkg)
		require.True(t, engine.IsInvalidInputError(err))
		require.Nil(t, reconstructor)
	})
}
