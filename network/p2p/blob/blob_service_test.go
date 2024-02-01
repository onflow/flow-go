package blob_test

import (
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	modmock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/p2p/blob"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAuthorizedRequester(t *testing.T) {
	providerData := map[peer.ID]*flow.Identity{}
	allowList := map[flow.Identifier]bool{}

	// known and on allow list
	an1, an1PeerID := mockIdentity(t, flow.RoleAccess)
	providerData[an1PeerID] = an1
	allowList[an1.NodeID] = true

	// known and not on allow list
	an2, an2PeerID := mockIdentity(t, flow.RoleAccess)
	providerData[an2PeerID] = an2

	// unknown and on the allow list
	an3, an3PeerID := mockIdentity(t, flow.RoleAccess)
	allowList[an3.NodeID] = true // should be ignored

	// known and on the allow list but not an access node
	sn1, sn1PeerID := mockIdentity(t, flow.RoleConsensus)
	providerData[sn1PeerID] = sn1
	allowList[sn1.NodeID] = true // should be ignored

	// known and should always be allowed
	en1, en1PeerID := mockIdentity(t, flow.RoleExecution)
	providerData[en1PeerID] = en1

	// unknown and should never be allowed
	en2, en2PeerID := mockIdentity(t, flow.RoleExecution)
	allowList[en2.NodeID] = true // should be ignored

	idProvider := modmock.NewIdentityProvider(t)
	idProvider.On("ByPeerID", mock.AnythingOfType("peer.ID")).Return(
		func(peerId peer.ID) *flow.Identity {
			return providerData[peerId]
		}, func(peerId peer.ID) bool {
			_, ok := providerData[peerId]
			return ok
		})

	// Allows requests from authorized nodes
	t.Run("allows all ANs with empty allow list", func(t *testing.T) {
		authorizer := blob.AuthorizedRequester(nil, idProvider, unittest.Logger())
		assert.True(t, authorizer(an1PeerID, cid.Cid{}), "AN1 should be allowed")
		assert.True(t, authorizer(an2PeerID, cid.Cid{}), "AN2 should be allowed")
	})

	t.Run("allows AN on allow list", func(t *testing.T) {
		authorizer := blob.AuthorizedRequester(allowList, idProvider, unittest.Logger())
		assert.True(t, authorizer(an1PeerID, cid.Cid{}))
	})

	t.Run("always allows EN", func(t *testing.T) {
		authorizer := blob.AuthorizedRequester(nil, idProvider, unittest.Logger())
		assert.True(t, authorizer(en1PeerID, cid.Cid{}))

		authorizer = blob.AuthorizedRequester(allowList, idProvider, unittest.Logger())
		assert.True(t, authorizer(en1PeerID, cid.Cid{}))
	})

	// Denies requests from nodes not on the allow list
	t.Run("denies AN not on allow list", func(t *testing.T) {
		authorizer := blob.AuthorizedRequester(allowList, idProvider, unittest.Logger())
		assert.False(t, authorizer(an2PeerID, cid.Cid{}))
	})

	// Denies requests from unknown nodes
	t.Run("never allow SN", func(t *testing.T) {
		authorizer := blob.AuthorizedRequester(nil, idProvider, unittest.Logger())
		assert.False(t, authorizer(sn1PeerID, cid.Cid{}))

		authorizer = blob.AuthorizedRequester(allowList, idProvider, unittest.Logger())
		assert.False(t, authorizer(sn1PeerID, cid.Cid{}))
	})

	an1.EpochParticipationStatus = flow.EpochParticipationStatusEjected
	en1.EpochParticipationStatus = flow.EpochParticipationStatusEjected

	// AN1 is on allow list (not passed) but is ejected
	t.Run("always denies ejected AN", func(t *testing.T) {
		authorizer := blob.AuthorizedRequester(nil, idProvider, unittest.Logger())
		assert.False(t, authorizer(an1PeerID, cid.Cid{}))

		authorizer = blob.AuthorizedRequester(allowList, idProvider, unittest.Logger())
		assert.False(t, authorizer(an1PeerID, cid.Cid{}))
	})

	// EN1 is on allow list (not passed) but is ejected
	t.Run("always denies ejected EN", func(t *testing.T) {
		authorizer := blob.AuthorizedRequester(nil, idProvider, unittest.Logger())
		assert.False(t, authorizer(en1PeerID, cid.Cid{}))

		authorizer = blob.AuthorizedRequester(allowList, idProvider, unittest.Logger())
		assert.False(t, authorizer(en1PeerID, cid.Cid{}))
	})

	// AN3 is on allow list (not passed) but is not in identity store
	t.Run("always denies unknown peer", func(t *testing.T) {
		authorizer := blob.AuthorizedRequester(nil, idProvider, unittest.Logger())
		assert.False(t, authorizer(an3PeerID, cid.Cid{}))
		assert.False(t, authorizer(en2PeerID, cid.Cid{}))

		authorizer = blob.AuthorizedRequester(allowList, idProvider, unittest.Logger())
		assert.False(t, authorizer(an3PeerID, cid.Cid{}))
		assert.False(t, authorizer(en2PeerID, cid.Cid{}))
	})
}

func mockIdentity(t *testing.T, role flow.Role) (*flow.Identity, peer.ID) {
	identity, _ := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(role))
	peerID, err := unittest.PeerIDFromFlowID(identity)
	require.NoError(t, err)

	return identity, peerID
}
