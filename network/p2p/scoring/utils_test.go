package scoring_test

import (
	"github.com/onflow/flow-go/model/flow"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestHasValidIdentity_Unknown tests that when a peer has an unknown identity, the HasValidIdentity returns InvalidPeerIDError
func TestHasValidIdentity_Unknown(t *testing.T) {
	peerId := p2pfixtures.PeerIdFixture(t)
	idProvider := mock.NewIdentityProvider(t)
	idProvider.On("ByPeerID", peerId).Return(nil, false)

	identity, err := scoring.HasValidFlowIdentity(idProvider, peerId)
	require.Nil(t, identity)
	require.Error(t, err)
	require.True(t, scoring.IsInvalidPeerIDError(err))
	require.Contains(t, err.Error(), scoring.PeerIdStatusUnknown)
}

// TestHasValidIdentity_Ejected tests that when a peer has been ejected, the HasValidIdentity returns an InvalidPeerIDError.
func TestHasValidIdentity_Ejected(t *testing.T) {
	idProvider := mock.NewIdentityProvider(t)

	ejectedIdentity := unittest.IdentityFixture()
	ejectedIdentity.EpochParticipationStatus = flow.EpochParticipationStatusEjected
	peerId := p2pfixtures.PeerIdFixture(t)
	idProvider.On("ByPeerID", peerId).Return(ejectedIdentity, true)

	identity, err := scoring.HasValidFlowIdentity(idProvider, peerId)
	require.Error(t, err)
	require.True(t, scoring.IsInvalidPeerIDError(err))
	require.Contains(t, err.Error(), scoring.PeerIdStatusEjected)
	require.Nil(t, identity)
}

// TestHasValidIdentity_valid tests that when a peer has a valid identity, the HasValidIdentity returns that identity.
func TestHasValidIdentity_Valid(t *testing.T) {
	idProvider := mock.NewIdentityProvider(t)

	trueID := unittest.IdentityFixture()
	peerId := p2pfixtures.PeerIdFixture(t)
	idProvider.On("ByPeerID", peerId).Return(trueID, true)

	identity, err := scoring.HasValidFlowIdentity(idProvider, peerId)
	require.NoError(t, err)
	require.Equal(t, trueID, identity)
}
