package signature

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	mockhotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestRandomBeaconReconstructor_InvalidSignerID tests that RandomBeaconReconstructor doesn't forward calls to
// RandomBeaconInspector if it fails to map signerID to signerIndex
func TestRandomBeaconReconstructor_InvalidSignerID(t *testing.T) {
	dkg := &mockhotstuff.DKG{}
	inspector := &mockhotstuff.RandomBeaconInspector{}
	reconstructor := NewRandomBeaconReconstructor(dkg, inspector)
	exception := errors.New("invalid-signer-id")
	t.Run("trustedAdd", func(t *testing.T) {
		signerID := unittest.IdentifierFixture()
		dkg.On("Index", signerID).Return(uint(0), exception).Once()
		_, err := reconstructor.TrustedAdd(signerID, unittest.SignatureFixture())
		require.ErrorAs(t, err, &exception)
		inspector.AssertNotCalled(t, "TrustedAdd")
	})
	t.Run("verify", func(t *testing.T) {
		signerID := unittest.IdentifierFixture()
		dkg.On("Index", signerID).Return(uint(0), exception).Once()
		err := reconstructor.Verify(signerID, unittest.SignatureFixture())
		require.ErrorAs(t, err, &exception)
		inspector.AssertNotCalled(t, "Verify")
	})

	dkg.AssertExpectations(t)
	inspector.AssertExpectations(t)
}
