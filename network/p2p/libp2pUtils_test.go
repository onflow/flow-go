package p2p

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
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/utils/unittest"
)

type LibP2PUtilsTestSuite struct {
	suite.Suite
}

func TestLibP2PUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(LibP2PUtilsTestSuite))
}

// TestPeerInfoFromID tests that PeerInfoFromID converts a flow.Identity to peer.AddrInfo correctly
func (ts *LibP2PUtilsTestSuite) TestPeerInfoFromID() {
	ids, exceptedPeerInfos := idsAndPeerInfos(ts.T())
	for i, id := range ids {
		actualAddrInfo, err := PeerAddressInfo(*id)
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
		libp2pKey, err := keyutils.LibP2PPublicKeyFromFlow(id.NetworkPubKey)
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

func BenchmarkPeerInfoFromID(b *testing.B) {
	b.StopTimer()
	id := unittest.IdentityFixture()
	key, _ := generateFlowNetworkingKey(id.NodeID)
	id.NetworkPubKey = key.PublicKey()
	id.Address = "1.1.1.1:3569"
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		_, _ = PeerAddressInfo(*id)
	}
}
