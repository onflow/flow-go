package signature

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	mockhotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
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

// TestNewRandomBeaconReconstructor
func TestNewRandomBeaconReconstructor(t *testing.T) {

}
