package test

import (
	"context"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/utils/unittest"
)

type PeerStoreProviderTestSuite struct {
	suite.Suite
	logger         zerolog.Logger
	nodes          []*p2p.Node
	libp2pPeersIDs []peer.ID
	peerIDprovider *p2p.PeerstoreIdentifierProvider
	translator     *p2p.HierarchicalIDTranslator
	ids            flow.IdentityList
}

func TestPeerStoreProviderTestSuite(t *testing.T) {
	suite.Run(t, new(PeerStoreProviderTestSuite))
}

const nodeCount = 2
const peerCount = 3
const testNodeIndex = 0 // testNodeIndex < nodeCount

func (suite *PeerStoreProviderTestSuite) SetupTest() {
	suite.logger = zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	log.SetAllLoggers(log.LevelError)
	ctx := context.Background()

	suite.ids, suite.nodes, _ = GenerateIDs(suite.T(), suite.logger, nodeCount, !DryRun, true, nil, nil)
	t, err := p2p.NewFixedTableIdentityTranslator(suite.ids)
	require.NoError(suite.T(), err)

	u := p2p.NewUnstakedNetworkIDTranslator()
	suite.translator = p2p.NewHierarchicalIDTranslator(t, u)

	// emulate the middleware behavior in populating the testnode's peer store
	libp2pPeers := make([]peer.ID, peerCount)
	for i := 0; i < peerCount; i++ {
		peerAddrInfo := suite.randomPeerInfoWithStubNetwork()
		err := suite.nodes[testNodeIndex].AddPeer(ctx, peerAddrInfo)
		// conn gater (then connection routine) will complain
		require.Error(suite.T(), err)
		libp2pPeers[i] = peerAddrInfo.ID
	}
	suite.libp2pPeersIDs = libp2pPeers
	suite.peerIDprovider = p2p.NewPeerstoreIdentifierProvider(suite.logger, suite.nodes[testNodeIndex].Host(), suite.translator)

	// sanity checks
	assert.Len(suite.T(), suite.nodes, nodeCount)
	assert.Len(suite.T(), suite.libp2pPeersIDs, peerCount)
	assert.Len(suite.T(), suite.ids, nodeCount)

}

func (suite *PeerStoreProviderTestSuite) TestTranslationPeers() {

	identifiers := suite.peerIDprovider.Identifiers()

	peerIDs := make([]peer.ID, len(identifiers))
	for i := 0; i < len(identifiers); i++ {

		pID, err := suite.translator.GetPeerID(identifiers[i])
		require.NoError(suite.T(), err)
		peerIDs[i] = pID
	}
	// check we can find the libp2p peers
	assert.ElementsMatch(suite.T(), peerIDs, append(suite.libp2pPeersIDs, suite.nodes[testNodeIndex].Host().ID()), "peer IDs should include those in the peer Store")

}

func (suite *PeerStoreProviderTestSuite) randomPeerInfoWithStubNetwork() peer.AddrInfo {

	// we don't care about network information, but those peers need an address
	ip := "127.0.0.1"
	port := strconv.Itoa(rand.Intn(65535 - 1024))

	addr := p2p.MultiAddressStr(ip, port)
	maddr, err := multiaddr.NewMultiaddr(addr)
	require.NoError(suite.T(), err)

	privKey, err := utils.GenerateUnstakedNetworkingKey(unittest.SeedFixture(crypto.KeyGenSeedMinLenECDSASecp256k1))
	require.NoError(suite.T(), err)

	libp2pKey, err := keyutils.LibP2PPublicKeyFromFlow(privKey.PublicKey())
	require.NoError(suite.T(), err)

	id, err := peer.IDFromPublicKey(libp2pKey)
	require.NoError(suite.T(), err)

	pInfo := peer.AddrInfo{ID: id, Addrs: []multiaddr.Multiaddr{maddr}}
	return pInfo
}
