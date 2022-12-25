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
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type NodeBlocklistWrapperTestSuite struct {
	suite.Suite
	DB       *badger.DB
	provider *mocks.IdentityProvider

	wrapper      *cache.NodeBlocklistWrapper
	mockNotifier *mockp2p.NodeBlockListConsumer
}

func (s *NodeBlocklistWrapperTestSuite) SetupTest() {
	s.DB, _ = unittest.TempBadgerDB(s.T())
	s.provider = new(mocks.IdentityProvider)

	var err error
	s.mockNotifier = mockp2p.NewNodeBlockListConsumer(s.T())
	s.wrapper, err = cache.NewNodeBlocklistWrapper(s.provider, s.DB, s.mockNotifier)
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
		expectedFilteredIdentities := identities.Filter(f)
		s.provider.On("Identities", mock.Anything).Return(
			func(filter flow.IdentityFilter) flow.IdentityList {
				return identities.Filter(filter)
			},
			nil,
		)
		require.Equal(s.T(), expectedFilteredIdentities, s.wrapper.Identities(f))
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
	s.mockNotifier.On("OnNodeBlockListUpdate", blocklist.NodeIDs()).Once()
	err := s.wrapper.Update(blocklist.NodeIDs())
	require.NoError(s.T(), err)

	index := atomic.NewInt32(0)
	for _, b := range []bool{true, false} {
		expectedfound := b

		s.Run(fmt.Sprintf("IdentityProvider.ByNodeID returning (<non-nil identity>, %v)", expectedfound), func() {
			originalIdentity := blocklist[index.Inc()]
			s.provider.On("ByNodeID", originalIdentity.NodeID).Return(originalIdentity, expectedfound)

			var expectedIdentity flow.Identity = *originalIdentity // expected Identity is a copy of the original
			expectedIdentity.Ejected = true                        // with the `Ejected` flag set to true

			i, found := s.wrapper.ByNodeID(originalIdentity.NodeID)
			require.Equal(s.T(), expectedfound, found)
			require.Equal(s.T(), &expectedIdentity, i)

			// check that originalIdentity returned by wrapped `IdentityProvider` is _not_ modified
			require.False(s.T(), originalIdentity.Ejected)
		})

		s.Run(fmt.Sprintf("IdentityProvider.ByPeerID returning (<non-nil identity>, %v)", expectedfound), func() {
			originalIdentity := blocklist[index.Inc()]
			peerID := (peer.ID)(originalIdentity.NodeID.String())
			s.provider.On("ByPeerID", peerID).Return(originalIdentity, expectedfound)

			var expectedIdentity flow.Identity = *originalIdentity // expected Identity is a copy of the original
			expectedIdentity.Ejected = true                        // with the `Ejected` flag set to true

			i, found := s.wrapper.ByPeerID(peerID)
			require.Equal(s.T(), expectedfound, found)
			require.Equal(s.T(), &expectedIdentity, i)

			// check that originalIdentity returned by `IdentityProvider` is _not_ modified by wrapper
			require.False(s.T(), originalIdentity.Ejected)
		})
	}

	s.Run("Identities", func() {
		blocklistLookup := blocklist.Lookup()
		honestIdentities := unittest.IdentityListFixture(8)
		combinedIdentities := honestIdentities.Union(blocklist)
		combinedIdentities = combinedIdentities.DeterministicShuffle(1234)
		numIdentities := len(combinedIdentities)

		s.provider.On("Identities", mock.Anything).Return(combinedIdentities)

		noFilter := filter.In(nil)
		identities := s.wrapper.Identities(noFilter)

		require.Equal(s.T(), numIdentities, len(identities)) // expected number resulting identities have the
		for _, i := range identities {
			_, isBlocked := blocklistLookup[i.NodeID]
			require.Equal(s.T(), isBlocked, i.Ejected)
		}

		// check that original `combinedIdentities` returned by `IdentityProvider` are _not_ modified by wrapper
		require.Equal(s.T(), numIdentities, len(combinedIdentities)) // length of list should not be modified by wrapper
		for _, i := range combinedIdentities {
			require.False(s.T(), i.Ejected) // Ejected flag should still have the original value (false here)
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

// TestBlocklistAddRemove checks that adding and subsequently removing a node from the blocklist
// it in combination a no-op. We test two scenarious
//   - Node whose original `Identity` has `Ejected = false`:
//     After adding the node to the blocklist and then removing it again, the `Ejected` should be false.
//   - Node whose original `Identity` has `Ejected = true`:
//     After adding the node to the blocklist and then removing it again, the `Ejected` should be still be true.
func (s *NodeBlocklistWrapperTestSuite) TestBlocklistAddRemove() {
	for _, originalEjected := range []bool{true, false} {
		s.Run(fmt.Sprintf("Add & remove node with Ejected = %v", originalEjected), func() {
			originalIdentity := unittest.IdentityFixture()
			originalIdentity.Ejected = originalEjected
			peerID := (peer.ID)(originalIdentity.NodeID.String())
			s.provider.On("ByNodeID", originalIdentity.NodeID).Return(originalIdentity, true)
			s.provider.On("ByPeerID", peerID).Return(originalIdentity, true)

			// step 1: before putting node on blocklist,
			// an Identity with `Ejected` equal to the original value should be returned
			i, found := s.wrapper.ByNodeID(originalIdentity.NodeID)
			require.True(s.T(), found)
			require.Equal(s.T(), originalEjected, i.Ejected)

			i, found = s.wrapper.ByPeerID(peerID)
			require.True(s.T(), found)
			require.Equal(s.T(), originalEjected, i.Ejected)

			// step 2: _after_ putting node on blocklist,
			// an Identity with `Ejected` equal to `true` should be returned
			s.mockNotifier.On("OnNodeBlockListUpdate", flow.IdentifierList{originalIdentity.NodeID}).Once()
			err := s.wrapper.Update(flow.IdentifierList{originalIdentity.NodeID})
			require.NoError(s.T(), err)

			i, found = s.wrapper.ByNodeID(originalIdentity.NodeID)
			require.True(s.T(), found)
			require.True(s.T(), i.Ejected)

			i, found = s.wrapper.ByPeerID(peerID)
			require.True(s.T(), found)
			require.True(s.T(), i.Ejected)

			// step 3: after removing the node from the blocklist,
			// an Identity with `Ejected` equal to the original value should be returned
			err = s.wrapper.Update(flow.IdentifierList{})
			require.NoError(s.T(), err)

			i, found = s.wrapper.ByNodeID(originalIdentity.NodeID)
			require.True(s.T(), found)
			require.Equal(s.T(), originalEjected, i.Ejected)

			i, found = s.wrapper.ByPeerID(peerID)
			require.True(s.T(), found)
			require.Equal(s.T(), originalEjected, i.Ejected)
		})
	}
}

// TestUpdate tests updating, clearing and retrieving the blocklist.
// This test verifies that the wrapper updates _its own internal state_ correctly.
// Note:
// conceptually, the blocklist is a set, i.e. not order dependent.
// The wrapper internally converts the list to a set and vice versa. Therefore
// the order is not preserved by `GetBlocklist`. Consequently, we compare
// map-based representations here.
func (s *NodeBlocklistWrapperTestSuite) TestUpdate() {
	blocklist1 := unittest.IdentifierListFixture(8)
	blocklist2 := unittest.IdentifierListFixture(11)
	blocklist3 := unittest.IdentifierListFixture(5)

	s.mockNotifier.On("OnNodeBlockListUpdate", blocklist1).Once()
	err := s.wrapper.Update(blocklist1)
	require.NoError(s.T(), err)
	require.Equal(s.T(), blocklist1.Lookup(), s.wrapper.GetBlocklist().Lookup())

	s.mockNotifier.On("OnNodeBlockListUpdate", blocklist2).Once()
	err = s.wrapper.Update(blocklist2)
	require.NoError(s.T(), err)
	require.Equal(s.T(), blocklist2.Lookup(), s.wrapper.GetBlocklist().Lookup())

	err = s.wrapper.ClearBlocklist()
	require.NoError(s.T(), err)
	require.Empty(s.T(), s.wrapper.GetBlocklist())

	s.mockNotifier.On("OnNodeBlockListUpdate", blocklist3).Once()
	err = s.wrapper.Update(blocklist3)
	require.NoError(s.T(), err)
	require.Equal(s.T(), blocklist3.Lookup(), s.wrapper.GetBlocklist().Lookup())
}

// TestDataBasePersist verifies database interactions of the wrapper with the data base.
// This test verifies that the blocklist updates are persisted across restarts.
// To decouple this test from the lower-level data base design, we proceed as follows:
//   - We do data-base operation through the exported methods from `NodeBlocklistWrapper`
//   - Then, we create a new `NodeBlocklistWrapper` backed by the same data base. Since it is a
//     new wrapper, it must read its state from the data base. Hence, if the new wrapper returns
//     the correct data, we have strong evidence that data-base interactions are correct.
//
// Note: The wrapper internally converts the list to a set and vice versa. Therefore
// the order is not preserved by `GetBlocklist`. Consequently, we compare
// map-based representations here.
func (s *NodeBlocklistWrapperTestSuite) TestDataBasePersist() {
	blocklist := unittest.IdentifierListFixture(8)
	blocklist2 := unittest.IdentifierListFixture(8)

	s.Run("Get blocklist from empty database", func() {
		require.Empty(s.T(), s.wrapper.GetBlocklist())
	})

	s.Run("Clear blocklist on empty database", func() {
		err := s.wrapper.ClearBlocklist() // No-op as data base does not contain any block list
		require.NoError(s.T(), err)
		require.Empty(s.T(), s.wrapper.GetBlocklist())

		// newly created wrapper should read `blocklist` from data base during initialization
		w, err := cache.NewNodeBlocklistWrapper(s.provider, s.DB, s.mockNotifier)
		require.NoError(s.T(), err)
		require.Empty(s.T(), w.GetBlocklist())
	})

	s.Run("Update blocklist and init new wrapper from database", func() {
		s.mockNotifier.On("OnNodeBlockListUpdate", blocklist).Once()
		err := s.wrapper.Update(blocklist)
		require.NoError(s.T(), err)

		// newly created wrapper should read `blocklist` from data base during initialization
		w, err := cache.NewNodeBlocklistWrapper(s.provider, s.DB, s.mockNotifier)
		require.NoError(s.T(), err)
		require.Equal(s.T(), blocklist.Lookup(), w.GetBlocklist().Lookup())
	})

	s.Run("Update and overwrite blocklist and then init new wrapper from database", func() {
		s.mockNotifier.On("OnNodeBlockListUpdate", blocklist).Once()
		err := s.wrapper.Update(blocklist)
		require.NoError(s.T(), err)

		s.mockNotifier.On("OnNodeBlockListUpdate", blocklist2).Once()
		err = s.wrapper.Update(blocklist2)
		require.NoError(s.T(), err)

		// newly created wrapper should read initial state from data base
		w, err := cache.NewNodeBlocklistWrapper(s.provider, s.DB, s.mockNotifier)
		require.NoError(s.T(), err)
		require.Equal(s.T(), blocklist2.Lookup(), w.GetBlocklist().Lookup())
	})

	s.Run("Update & clear & update and then init new wrapper from database", func() {
		// set blocklist ->
		// newly created wrapper should now read this list from data base during initialization
		s.mockNotifier.On("OnNodeBlockListUpdate", blocklist).Once()
		err := s.wrapper.Update(blocklist)
		require.NoError(s.T(), err)

		w0, err := cache.NewNodeBlocklistWrapper(s.provider, s.DB, s.mockNotifier)
		require.NoError(s.T(), err)
		require.Equal(s.T(), blocklist.Lookup(), w0.GetBlocklist().Lookup())

		// clear blocklist ->
		// newly created wrapper should now read empty blocklist from data base during initialization
		err = s.wrapper.ClearBlocklist()
		require.NoError(s.T(), err)

		w1, err := cache.NewNodeBlocklistWrapper(s.provider, s.DB, s.mockNotifier)
		require.NoError(s.T(), err)
		require.Empty(s.T(), w1.GetBlocklist())

		// set blocklist2 ->
		// newly created wrapper should now read this list from data base during initialization
		s.mockNotifier.On("OnNodeBlockListUpdate", blocklist2).Once()
		err = s.wrapper.Update(blocklist2)
		require.NoError(s.T(), err)

		w2, err := cache.NewNodeBlocklistWrapper(s.provider, s.DB, s.mockNotifier)
		require.NoError(s.T(), err)
		require.Equal(s.T(), blocklist2.Lookup(), w2.GetBlocklist().Lookup())
	})
}
