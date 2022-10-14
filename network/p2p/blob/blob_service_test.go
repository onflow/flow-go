package blob

import (
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	modmock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAuthorizedRequester(t *testing.T) {
	providerData := map[peer.ID]*flow.Identity{}
	allowList := map[flow.Identifier]bool{}

	// known and on allow list
	an1, an1PeerID := identityInfo(t, flow.RoleAccess)
	providerData[an1PeerID] = an1
	allowList[an1.NodeID] = true

	// known and not on allow list
	an2, an2PeerID := identityInfo(t, flow.RoleAccess)
	providerData[an2PeerID] = an2

	// unknown and on the allow list
	an3, an3PeerID := identityInfo(t, flow.RoleAccess)
	allowList[an3.NodeID] = true // should be ignored

	// known and on the allow list but not an access node
	sn1, sn1PeerID := identityInfo(t, flow.RoleConsensus)
	providerData[sn1PeerID] = sn1
	allowList[sn1.NodeID] = true // should be ignored

	idProvider := modmock.NewIdentityProvider(t)
	idProvider.On("ByPeerID", mock.AnythingOfType("peer.ID")).Return(
		func(peerId peer.ID) *flow.Identity {
			identity, _ := providerData[peerId]
			return identity
		}, func(peerId peer.ID) bool {
			_, ok := providerData[peerId]
			return ok
		})

	t.Run("allows AN without allow list", func(t *testing.T) {
		authorizer := AuthorizedRequester(nil, idProvider, unittest.Logger())
		assert.True(t, authorizer(an1PeerID, cid.Cid{}))
	})

	t.Run("allows AN on allow list", func(t *testing.T) {
		authorizer := AuthorizedRequester(allowList, idProvider, unittest.Logger())
		assert.True(t, authorizer(an1PeerID, cid.Cid{}))
	})

	t.Run("denies AN not on allow list", func(t *testing.T) {
		authorizer := AuthorizedRequester(allowList, idProvider, unittest.Logger())
		assert.False(t, authorizer(an2PeerID, cid.Cid{}))
	})

	t.Run("denies SN without allow list", func(t *testing.T) {
		authorizer := AuthorizedRequester(nil, idProvider, unittest.Logger())
		assert.False(t, authorizer(sn1PeerID, cid.Cid{}))
	})

	t.Run("denies SN with allow list", func(t *testing.T) {
		authorizer := AuthorizedRequester(allowList, idProvider, unittest.Logger())
		assert.False(t, authorizer(sn1PeerID, cid.Cid{}))
	})

	an1.Ejected = true

	// AN1 is on allow list (not passed) but is ejected
	t.Run("denies ejected AN without allow list", func(t *testing.T) {
		authorizer := AuthorizedRequester(nil, idProvider, unittest.Logger())
		assert.False(t, authorizer(an1PeerID, cid.Cid{}))
	})

	// AN1 is on allow list but is ejected
	t.Run("denies ejected AN with allow list", func(t *testing.T) {
		authorizer := AuthorizedRequester(allowList, idProvider, unittest.Logger())
		assert.False(t, authorizer(an1PeerID, cid.Cid{}))
	})

	// AN3 is on allow list (not passed) but is not in identity store
	t.Run("denies unknown peer without allow list", func(t *testing.T) {
		authorizer := AuthorizedRequester(nil, idProvider, unittest.Logger())
		assert.False(t, authorizer(an3PeerID, cid.Cid{}))
	})

	// AN3 is on allow list but is not in identity store
	t.Run("denies unknown peer with allow list", func(t *testing.T) {
		authorizer := AuthorizedRequester(allowList, idProvider, unittest.Logger())
		assert.False(t, authorizer(an3PeerID, cid.Cid{}))
	})
}

func identityInfo(t *testing.T, role flow.Role) (*flow.Identity, peer.ID) {
	identity, _ := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(role))
	peerID, err := unittest.PeerIDFromFlowID(identity)
	require.NoError(t, err)

	return identity, peerID
}
