package cache_test

import (
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	mocks "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/p2p/cache"
	"github.com/onflow/flow-go/utils/unittest"
)

type NodeBlocklistWrapperTestSuite struct {
	suite.Suite
	DB       *badger.DB
	provider *mocks.IdentityProvider

	wrapper *cache.NodeBlocklistWrapper
}

func (s *NodeBlocklistWrapperTestSuite) SetupTest() {
	s.DB, _ = unittest.TempBadgerDB(s.T())
	s.provider = new(mocks.IdentityProvider)

	var err error
	s.wrapper, err = cache.NewNodeBlocklistWrapper(s.provider, s.DB)
	require.NoError(s.T(), err)
}

func TestNodeBlocklistWrapperTestSuite(t *testing.T) {
	suite.Run(t, new(NodeBlocklistWrapperTestSuite))
}

// TestHonestNode verifies:
// For nodes _not_ on the blocklist, the `cache.NodeBlocklistWrapper` should forward
// the identities from the wrapped `IdentityProvider` without modification.
func (s *NodeBlocklistWrapperTestSuite) TestHonestNode() {
	s.Run("ByNodeID", func() {
		identity := unittest.IdentityFixture()
		s.provider.On("ByNodeID", identity.NodeID).Return(identity, true)

		i, found := s.wrapper.ByNodeID(identity.NodeID)
		require.True(s.T(), found)
		require.Equal(s.T(), i, identity)
	})
	s.Run("ByPeerID", func() {
		identity := unittest.IdentityFixture()
		peerID := (peer.ID)("some_peer_ID")
		s.provider.On("ByPeerID", peerID).Return(identity, true)

		i, found := s.wrapper.ByPeerID(peerID)
		require.True(s.T(), found)
		require.Equal(s.T(), i, identity)
	})
	s.Run("Identities", func() {
		identities := unittest.IdentityListFixture(11)
		f := filter.In(identities[3:4])
		expectedFilteredIdentities := identities.Filter(filter)
		s.provider.On("Identities", mock.Anything).Return(
			func(filter flow.IdentityFilter) flow.IdentityList {
				return identities.Filter(filter)
			},
			nil,
		)
		require.Equal(s.T(), expectedFilteredIdentities, s.wrapper.Identities(filter))
	})
}

// TestBlacklistedNode tests proper handling of identities _on_ the blocklist:
//   - For any identity `i` with `i.NodeID ∈ blocklist`, the returned identity
//     should have `i.Ejected` set to `true` (irrespective of the `Ejected`
//     flag's initial returned by the wrapped `IdentityProvider`).
//   - The wrapper should _copy_ the identity and _not_ write into the wrapped
//     IdentityProvider's memory.
//   - For `IdentityProvider.ByNodeID` and `IdentityProvider.ByPeerID`:
//     whether or not the wrapper modifies the `Ejected` flag should depend only
//     in the NodeID of the returned identity, irrespective of the second return
//     value (boolean).
//     While returning (non-nil identity, false) is not a defined return value,
//     we expect the wrapper to nevertheless handle this case to increase its
//     generality.
func (s *NodeBlocklistWrapperTestSuite) TestBlacklistedNode() {
	blocklist := unittest.IdentityListFixture(11)
	err := s.wrapper.Update(blocklist.NodeIDs())
	require.NoError(s.T(), err)

	index := atomic.NewInt32(0)
	for _, b := range []bool{true, false} {
		expectedfound := b

		s.Run(fmt.Sprintf("IdentityProvider.ByNodeID returning (<non-nil identity>, %v)", expectedfound), func() {
			identity := blocklist[index.Inc()]
			s.provider.On("ByNodeID", identity.NodeID).Return(identity, expectedfound)

			var backupIdentity flow.Identity = *identity   // unmodified backup Identity
			var expectedIdentity flow.Identity = *identity // expected Identity is a copy of the original
			expectedIdentity.Ejected = true                // with the `Ejected` flag set to true

			i, found := s.wrapper.ByNodeID(identity.NodeID)
			require.Equal(s.T(), expectedfound, found)
			require.Equal(s.T(), &expectedIdentity, i)
			require.Equal(s.T(), &backupIdentity, identity) // verify that identity returned by wrapped `IdentityProvider` is not modified
		})

		s.Run(fmt.Sprintf("IdentityProvider.ByPeerID returning (<non-nil identity>, %v)", expectedfound), func() {
			identity := blocklist[index.Inc()]
			peerID := (peer.ID)(identity.NodeID.String())
			s.provider.On("ByPeerID", peerID).Return(identity, expectedfound)

			var backupIdentity flow.Identity = *identity   // unmodified backup Identity
			var expectedIdentity flow.Identity = *identity // expected Identity is a copy of the original
			expectedIdentity.Ejected = true                // with the `Ejected` flag set to true

			i, found := s.wrapper.ByPeerID(peerID)
			require.Equal(s.T(), expectedfound, found)
			require.Equal(s.T(), &expectedIdentity, i)
			require.Equal(s.T(), &backupIdentity, identity) // verify that identity returned by wrapped `IdentityProvider` is not modified
		})
	}

	s.Run("Identities", func() {
		blocklistLookup := blocklist.Lookup()
		honestIdentities := unittest.IdentityListFixture(8)
		combinedIdentities := honestIdentities.Union(blocklist)
		combinedIdentities = combinedIdentities.DeterministicShuffle(1234)
		s.provider.On("Identities", mock.Anything).Return(combinedIdentities)

		noFilter := filter.In(nil)
		identities := s.wrapper.Identities(noFilter)

		require.Equal(s.T(), len(combinedIdentities), len(identities))
		for _, i := range identities {
			_, isBlocked := blocklistLookup[i.NodeID]
			require.Equal(s.T(), isBlocked, i.Ejected)
		}
	})
}

// TestUnknownNode verifies that the wrapper forwards nil identities
// irrespective of the boolean return values.
func (s *NodeBlocklistWrapperTestSuite) TestUnknownNode() {
	for _, b := range []bool{true, false} {
		s.Run(fmt.Sprintf("IdentityProvider.ByNodeID returning (nil, %v)", b), func() {
			id := unittest.IdentifierFixture()
			s.provider.On("ByNodeID", id).Return(nil, b)

			identity, found := s.wrapper.ByNodeID(id)
			require.Equal(s.T(), b, found)
			require.Nil(s.T(), identity)
		})

		s.Run(fmt.Sprintf("IdentityProvider.ByPeerID returning (nil, %v)", b), func() {
			peerID := (peer.ID)(unittest.IdentifierFixture().String())
			s.provider.On("ByPeerID", peerID).Return(nil, b)

			identity, found := s.wrapper.ByPeerID(peerID)
			require.Equal(s.T(), b, found)
			require.Nil(s.T(), identity)
		})
	}
}
