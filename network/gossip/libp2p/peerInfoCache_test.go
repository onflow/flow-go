package libp2p

import (
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type PeerInfoCacheTestSuite struct {
	suite.Suite
}

func TestPeerInfoCacheTestSuite(t *testing.T) {
	suite.Run(t, new(PeerInfoCacheTestSuite))
}

func (ts *PeerInfoCacheTestSuite) SetupTest() {
	require.NoError(ts.T(), InitializePeerInfoCache())
}

// TestPeerInfoFromID tests that PeerInfoFromID converts a flow.Identity to peer.AddrInfo correctly
func (ts *PeerInfoCacheTestSuite) TestPeerInfoFromID() {
	ids, exceptedPeerInfos := idsAndPeerInfos(ts.T())
	for i, id := range ids {
		actualAddrInfo, err := PeerInfoFromID(*id)
		assert.NoError(ts.T(), err)
		assert.Equal(ts.T(), exceptedPeerInfos[i].String(), actualAddrInfo.String())
	}
}

// TestPeerInfoFromIDCacheHits tests that peer.AddrInfos are cached once generated and always returned from cache
// after that
func (ts *PeerInfoCacheTestSuite) TestPeerInfoFromIDCacheHits() {

	// assert that cache is empty
	assert.Zero(ts.T(), infoCache.Len())

	// generate test ids and equivalent peer.AddrInfo
	ids, exceptedPeerInfos := idsAndPeerInfos(ts.T())

	// convert ids to peer.AddrInfo
	for _, id := range ids {
		_, err := PeerInfoFromID(*id)
		assert.NoError(ts.T(), err)
	}

	// assert that cache now has expected number of entries
	assert.Equal(ts.T(), infoCache.Len(), len(ids))

	// change the address of each of the id
	for i := range ids {
		ids[i].Address = "3.3.3.3"
	}

	// the peer.AddrInfo should still match ones retrieved earlier, asserting that the value came from the cache
	for i, id := range ids {
		actualAddrInfo, err := PeerInfoFromID(*id)
		assert.NoError(ts.T(), err)
		assert.Equal(ts.T(), exceptedPeerInfos[i].String(), actualAddrInfo.String())
	}
}

func idsAndPeerInfos(t *testing.T) (flow.IdentityList, []peer.AddrInfo) {
	ips := []string{"1.1.1.1", "2.2.2.2"}
	port := "3569"

	ids := make(flow.IdentityList, len(ips))
	peerInfos := make([]peer.AddrInfo, len(ips))

	keyOpt := func(id *flow.Identity) {
		key, err := generateFlowNetworkingKey(id.NodeID)
		require.NoError(t, err)
		id.NetworkPubKey = key.PublicKey()
	}

	for i, ip := range ips {
		// create a flow Identity
		id := unittest.IdentityFixture(keyOpt)
		id.Address = fmt.Sprintf("%s:%s", ip, port)
		ids[i] = id

		// create a libp2p PeerAddressInfo
		libp2pKey, err := PublicKey(id.NetworkPubKey)
		assert.NoError(t, err)
		peerID, err := peer.IDFromPublicKey(libp2pKey)
		assert.NoError(t, err)
		addrInfo := peer.AddrInfo{
			ID:    peerID,
			Addrs: []multiaddr.Multiaddr{multiaddr.StringCast(fmt.Sprintf("/ip4/%s/tcp/%s", ip, port))},
		}
		peerInfos[i] = addrInfo
	}

	return ids, peerInfos
}

func generateFlowNetworkingKey(s flow.Identifier) (crypto.PrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenECDSASecp256k1)
	copy(seed, s[:])
	return crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
}
