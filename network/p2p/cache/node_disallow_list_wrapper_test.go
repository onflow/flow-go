package cache_test

import (
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	mocks "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p/cache"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

type NodeDisallowListWrapperTestSuite struct {
	suite.Suite
	DB       storage.DB
	provider *mocks.IdentityProvider

	wrapper        *cache.NodeDisallowListWrapper
	updateConsumer *mocknetwork.DisallowListNotificationConsumer
}

func newNodeDisallowListWrapperTestSuite(db storage.DB) *NodeDisallowListWrapperTestSuite {
	return &NodeDisallowListWrapperTestSuite{
		DB: db,
	}
}

func (s *NodeDisallowListWrapperTestSuite) SetupTest() {
	s.provider = new(mocks.IdentityProvider)

	var err error
	s.updateConsumer = mocknetwork.NewDisallowListNotificationConsumer(s.T())
	s.wrapper, err = cache.NewNodeDisallowListWrapper(s.provider, s.DB, func() network.DisallowListNotificationConsumer {
		return s.updateConsumer
	})
	require.NoError(s.T(), err)
}

func TestNodeDisallowListWrapperWithBadgerTestSuite(t *testing.T) {
	pdb, _ := unittest.TempPebbleDB(t)
	suite.Run(t, newNodeDisallowListWrapperTestSuite(pebbleimpl.ToDB(pdb)))
}

func TestNodeDisallowListWrapperWithPebbleTestSuite(t *testing.T) {
	pdb, _ := unittest.TempPebbleDB(t)
	suite.Run(t, newNodeDisallowListWrapperTestSuite(pebbleimpl.ToDB(pdb)))
}

// TestHonestNode verifies:
// For nodes _not_ on the disallowList, the `cache.NodeDisallowListingWrapper` should forward
// the identities from the wrapped `IdentityProvider` without modification.
func (s *NodeDisallowListWrapperTestSuite) TestHonestNode() {
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
			func(filter flow.IdentityFilter[flow.Identity]) flow.IdentityList {
				return identities.Filter(filter)
			},
			nil,
		)
		require.Equal(s.T(), expectedFilteredIdentities, s.wrapper.Identities(f))
	})
}

// TestDisallowListNode tests proper handling of identities _on_ the disallowList:
//   - For any identity `i` with `i.NodeID ∈ disallowList`, the returned identity
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
func (s *NodeDisallowListWrapperTestSuite) TestDisallowListNode() {
	disallowlist := unittest.IdentityListFixture(11)
	s.updateConsumer.On("OnDisallowListNotification", &network.DisallowListingUpdate{
		FlowIds: disallowlist.NodeIDs(),
		Cause:   network.DisallowListedCauseAdmin,
	}).Return().Once()
	err := s.wrapper.Update(disallowlist.NodeIDs())
	require.NoError(s.T(), err)

	index := atomic.NewInt32(0)
	for _, b := range []bool{true, false} {
		expectedfound := b

		s.Run(fmt.Sprintf("IdentityProvider.ByNodeID returning (<non-nil identity>, %v)", expectedfound), func() {
			originalIdentity := disallowlist[index.Inc()]
			s.provider.On("ByNodeID", originalIdentity.NodeID).Return(originalIdentity, expectedfound)

			var expectedIdentity = *originalIdentity                                         // expected Identity is a copy of the original
			expectedIdentity.EpochParticipationStatus = flow.EpochParticipationStatusEjected // with the `Ejected` flag set to true

			i, found := s.wrapper.ByNodeID(originalIdentity.NodeID)
			require.Equal(s.T(), expectedfound, found)
			require.Equal(s.T(), &expectedIdentity, i)

			// check that originalIdentity returned by wrapped `IdentityProvider` is _not_ modified
			require.False(s.T(), originalIdentity.IsEjected())
		})

		s.Run(fmt.Sprintf("IdentityProvider.ByPeerID returning (<non-nil identity>, %v)", expectedfound), func() {
			originalIdentity := disallowlist[index.Inc()]
			peerID := (peer.ID)(originalIdentity.NodeID.String())
			s.provider.On("ByPeerID", peerID).Return(originalIdentity, expectedfound)

			var expectedIdentity = *originalIdentity                                         // expected Identity is a copy of the original
			expectedIdentity.EpochParticipationStatus = flow.EpochParticipationStatusEjected // with the `Ejected` flag set to true

			i, found := s.wrapper.ByPeerID(peerID)
			require.Equal(s.T(), expectedfound, found)
			require.Equal(s.T(), &expectedIdentity, i)

			// check that originalIdentity returned by `IdentityProvider` is _not_ modified by wrapper
			require.False(s.T(), originalIdentity.IsEjected())
		})
	}

	s.Run("Identities", func() {
		disallowlistLookup := disallowlist.Lookup()
		honestIdentities := unittest.IdentityListFixture(8)
		combinedIdentities := honestIdentities.Union(disallowlist)
		combinedIdentities, err = combinedIdentities.Shuffle()
		require.NoError(s.T(), err)
		numIdentities := len(combinedIdentities)

		s.provider.On("Identities", mock.Anything).Return(combinedIdentities)

		noFilter := filter.Not(filter.In[flow.Identity](nil))
		identities := s.wrapper.Identities(noFilter)

		require.Equal(s.T(), numIdentities, len(identities)) // expected number resulting identities have the
		for _, i := range identities {
			_, isBlocked := disallowlistLookup[i.NodeID]
			require.Equal(s.T(), isBlocked, i.IsEjected())
		}

		// check that original `combinedIdentities` returned by `IdentityProvider` are _not_ modified by wrapper
		require.Equal(s.T(), numIdentities, len(combinedIdentities)) // length of list should not be modified by wrapper
		for _, i := range combinedIdentities {
			require.False(s.T(), i.IsEjected()) // Ejected flag should still have the original value (false here)
		}
	})

	// this tests the edge case where the  Identities func is invoked with the p2p.NotEjectedFilter. Block listed
	// nodes are expected to be filtered from the identity list returned after setting the ejected field.
	s.Run("Identities(p2p.NotEjectedFilter) should not return block listed nodes", func() {
		disallowlistLookup := disallowlist.Lookup()
		honestIdentities := unittest.IdentityListFixture(8)
		combinedIdentities := honestIdentities.Union(disallowlist)
		combinedIdentities, err = combinedIdentities.Shuffle()
		require.NoError(s.T(), err)
		numIdentities := len(combinedIdentities)

		s.provider.On("Identities", mock.Anything).Return(combinedIdentities)

		identities := s.wrapper.Identities(filter.NotEjectedFilter)

		require.Equal(s.T(), len(honestIdentities), len(identities)) // expected only honest nodes to be returned
		for _, i := range identities {
			_, isBlocked := disallowlistLookup[i.NodeID]
			require.False(s.T(), isBlocked)
			require.False(s.T(), i.IsEjected())
		}

		// check that original `combinedIdentities` returned by `IdentityProvider` are _not_ modified by wrapper
		require.Equal(s.T(), numIdentities, len(combinedIdentities)) // length of list should not be modified by wrapper
		for _, i := range combinedIdentities {
			require.False(s.T(), i.IsEjected()) // Ejected flag should still have the original value (false here)
		}
	})
}

// TestUnknownNode verifies that the wrapper forwards nil identities
// irrespective of the boolean return values.
func (s *NodeDisallowListWrapperTestSuite) TestUnknownNode() {
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

// TestDisallowListAddRemove checks that adding and subsequently removing a node from the disallowList
// it in combination a no-op. We test two scenarious
//   - Node whose original `Identity` has `Ejected = false`:
//     After adding the node to the disallowList and then removing it again, the `Ejected` should be false.
//   - Node whose original `Identity` has `EpochParticipationStatus = flow.EpochParticipationStatusEjected`:
//     After adding the node to the disallowList and then removing it again, the `Ejected` should be still be true.
func (s *NodeDisallowListWrapperTestSuite) TestDisallowListAddRemove() {
	for _, originalParticipationStatus := range []flow.EpochParticipationStatus{flow.EpochParticipationStatusEjected, flow.EpochParticipationStatusActive} {
		s.Run(fmt.Sprintf("Add & remove node with EpochParticipationStatus = %v", originalParticipationStatus), func() {
			originalIdentity := unittest.IdentityFixture()
			originalIdentity.EpochParticipationStatus = originalParticipationStatus
			peerID := (peer.ID)(originalIdentity.NodeID.String())
			s.provider.On("ByNodeID", originalIdentity.NodeID).Return(originalIdentity, true)
			s.provider.On("ByPeerID", peerID).Return(originalIdentity, true)

			// step 1: before putting node on disallowList,
			// an Identity with `Ejected` equal to the original value should be returned
			i, found := s.wrapper.ByNodeID(originalIdentity.NodeID)
			require.True(s.T(), found)
			require.Equal(s.T(), originalParticipationStatus, i.EpochParticipationStatus)

			i, found = s.wrapper.ByPeerID(peerID)
			require.True(s.T(), found)
			require.Equal(s.T(), originalParticipationStatus, i.EpochParticipationStatus)

			// step 2: _after_ putting node on disallowList,
			// an Identity with `Ejected` equal to `true` should be returned
			s.updateConsumer.On("OnDisallowListNotification", &network.DisallowListingUpdate{
				FlowIds: flow.IdentifierList{originalIdentity.NodeID},
				Cause:   network.DisallowListedCauseAdmin,
			}).Return().Once()
			err := s.wrapper.Update(flow.IdentifierList{originalIdentity.NodeID})
			require.NoError(s.T(), err)

			i, found = s.wrapper.ByNodeID(originalIdentity.NodeID)
			require.True(s.T(), found)
			require.True(s.T(), i.IsEjected())

			i, found = s.wrapper.ByPeerID(peerID)
			require.True(s.T(), found)
			require.True(s.T(), i.IsEjected())

			// step 3: after removing the node from the disallowList,
			// an Identity with `Ejected` equal to the original value should be returned
			s.updateConsumer.On("OnDisallowListNotification", &network.DisallowListingUpdate{
				FlowIds: flow.IdentifierList{},
				Cause:   network.DisallowListedCauseAdmin,
			}).Return().Once()
			err = s.wrapper.Update(flow.IdentifierList{})
			require.NoError(s.T(), err)

			i, found = s.wrapper.ByNodeID(originalIdentity.NodeID)
			require.True(s.T(), found)
			require.Equal(s.T(), originalParticipationStatus, i.EpochParticipationStatus)

			i, found = s.wrapper.ByPeerID(peerID)
			require.True(s.T(), found)
			require.Equal(s.T(), originalParticipationStatus, i.EpochParticipationStatus)
		})
	}
}

// TestUpdate tests updating, clearing and retrieving the disallowList.
// This test verifies that the wrapper updates _its own internal state_ correctly.
// Note:
// conceptually, the disallowList is a set, i.e. not order dependent.
// The wrapper internally converts the list to a set and vice versa. Therefore
// the order is not preserved by `GetDisallowList`. Consequently, we compare
// map-based representations here.
func (s *NodeDisallowListWrapperTestSuite) TestUpdate() {
	disallowList1 := unittest.IdentifierListFixture(8)
	disallowList2 := unittest.IdentifierListFixture(11)
	disallowList3 := unittest.IdentifierListFixture(5)

	s.updateConsumer.On("OnDisallowListNotification", &network.DisallowListingUpdate{
		FlowIds: disallowList1,
		Cause:   network.DisallowListedCauseAdmin,
	}).Return().Once()
	err := s.wrapper.Update(disallowList1)
	require.NoError(s.T(), err)
	require.Equal(s.T(), disallowList1.Lookup(), s.wrapper.GetDisallowList().Lookup())

	s.updateConsumer.On("OnDisallowListNotification", &network.DisallowListingUpdate{
		FlowIds: disallowList2,
		Cause:   network.DisallowListedCauseAdmin,
	}).Return().Once()
	err = s.wrapper.Update(disallowList2)
	require.NoError(s.T(), err)
	require.Equal(s.T(), disallowList2.Lookup(), s.wrapper.GetDisallowList().Lookup())

	s.updateConsumer.On("OnDisallowListNotification", &network.DisallowListingUpdate{
		FlowIds: nil,
		Cause:   network.DisallowListedCauseAdmin,
	}).Return().Once()
	err = s.wrapper.ClearDisallowList()
	require.NoError(s.T(), err)
	require.Empty(s.T(), s.wrapper.GetDisallowList())

	s.updateConsumer.On("OnDisallowListNotification", &network.DisallowListingUpdate{
		FlowIds: disallowList3,
		Cause:   network.DisallowListedCauseAdmin,
	}).Return().Once()
	err = s.wrapper.Update(disallowList3)
	require.NoError(s.T(), err)
	require.Equal(s.T(), disallowList3.Lookup(), s.wrapper.GetDisallowList().Lookup())
}

// TestDataBasePersist verifies database interactions of the wrapper with the data base.
// This test verifies that the disallowList updates are persisted across restarts.
// To decouple this test from the lower-level data base design, we proceed as follows:
//   - We do data-base operation through the exported methods from `NodeDisallowListingWrapper`
//   - Then, we create a new `NodeDisallowListingWrapper` backed by the same data base. Since it is a
//     new wrapper, it must read its state from the data base. Hence, if the new wrapper returns
//     the correct data, we have strong evidence that data-base interactions are correct.
//
// Note: The wrapper internally converts the list to a set and vice versa. Therefore
// the order is not preserved by `GetDisallowList`. Consequently, we compare
// map-based representations here.
func (s *NodeDisallowListWrapperTestSuite) TestDataBasePersist() {
	disallowList1 := unittest.IdentifierListFixture(8)
	disallowList2 := unittest.IdentifierListFixture(8)

	s.Run("Get disallowList from empty database", func() {
		require.Empty(s.T(), s.wrapper.GetDisallowList())
	})

	s.Run("Clear disallow-list on empty database", func() {
		s.updateConsumer.On("OnDisallowListNotification", &network.DisallowListingUpdate{
			FlowIds: nil,
			Cause:   network.DisallowListedCauseAdmin,
		}).Return().Once()
		err := s.wrapper.ClearDisallowList() // No-op as data base does not contain any block list
		require.NoError(s.T(), err)
		require.Empty(s.T(), s.wrapper.GetDisallowList())

		// newly created wrapper should read `disallowList` from data base during initialization
		w, err := cache.NewNodeDisallowListWrapper(s.provider, s.DB, func() network.DisallowListNotificationConsumer {
			return s.updateConsumer
		})
		require.NoError(s.T(), err)
		require.Empty(s.T(), w.GetDisallowList())
	})

	s.Run("Update disallowList and init new wrapper from database", func() {
		s.updateConsumer.On("OnDisallowListNotification", &network.DisallowListingUpdate{
			FlowIds: disallowList1,
			Cause:   network.DisallowListedCauseAdmin,
		}).Return().Once()
		err := s.wrapper.Update(disallowList1)
		require.NoError(s.T(), err)

		// newly created wrapper should read `disallowList` from data base during initialization
		w, err := cache.NewNodeDisallowListWrapper(s.provider, s.DB, func() network.DisallowListNotificationConsumer {
			return s.updateConsumer
		})
		require.NoError(s.T(), err)
		require.Equal(s.T(), disallowList1.Lookup(), w.GetDisallowList().Lookup())
	})

	s.Run("Update and overwrite disallowList and then init new wrapper from database", func() {
		s.updateConsumer.On("OnDisallowListNotification", &network.DisallowListingUpdate{
			FlowIds: disallowList1,
			Cause:   network.DisallowListedCauseAdmin,
		}).Return().Once()
		err := s.wrapper.Update(disallowList1)
		require.NoError(s.T(), err)

		s.updateConsumer.On("OnDisallowListNotification", &network.DisallowListingUpdate{
			FlowIds: disallowList2,
			Cause:   network.DisallowListedCauseAdmin,
		}).Return().Once()
		err = s.wrapper.Update(disallowList2)
		require.NoError(s.T(), err)

		// newly created wrapper should read initial state from data base
		w, err := cache.NewNodeDisallowListWrapper(s.provider, s.DB, func() network.DisallowListNotificationConsumer {
			return s.updateConsumer
		})
		require.NoError(s.T(), err)
		require.Equal(s.T(), disallowList2.Lookup(), w.GetDisallowList().Lookup())
	})

	s.Run("Update & clear & update and then init new wrapper from database", func() {
		// set disallowList ->
		// newly created wrapper should now read this list from data base during initialization
		s.updateConsumer.On("OnDisallowListNotification", &network.DisallowListingUpdate{
			FlowIds: disallowList1,
			Cause:   network.DisallowListedCauseAdmin,
		}).Return().Once()
		err := s.wrapper.Update(disallowList1)
		require.NoError(s.T(), err)

		w0, err := cache.NewNodeDisallowListWrapper(s.provider, s.DB, func() network.DisallowListNotificationConsumer {
			return s.updateConsumer
		})
		require.NoError(s.T(), err)
		require.Equal(s.T(), disallowList1.Lookup(), w0.GetDisallowList().Lookup())

		// clear disallowList ->
		// newly created wrapper should now read empty disallowList from data base during initialization
		s.updateConsumer.On("OnDisallowListNotification", &network.DisallowListingUpdate{
			FlowIds: nil,
			Cause:   network.DisallowListedCauseAdmin,
		}).Return().Once()
		err = s.wrapper.ClearDisallowList()
		require.NoError(s.T(), err)

		w1, err := cache.NewNodeDisallowListWrapper(s.provider, s.DB, func() network.DisallowListNotificationConsumer {
			return s.updateConsumer
		})
		require.NoError(s.T(), err)
		require.Empty(s.T(), w1.GetDisallowList())

		// set disallowList2 ->
		// newly created wrapper should now read this list from data base during initialization
		s.updateConsumer.On("OnDisallowListNotification", &network.DisallowListingUpdate{
			FlowIds: disallowList2,
			Cause:   network.DisallowListedCauseAdmin,
		}).Return().Once()
		err = s.wrapper.Update(disallowList2)
		require.NoError(s.T(), err)

		w2, err := cache.NewNodeDisallowListWrapper(s.provider, s.DB, func() network.DisallowListNotificationConsumer {
			return s.updateConsumer
		})
		require.NoError(s.T(), err)
		require.Equal(s.T(), disallowList2.Lookup(), w2.GetDisallowList().Lookup())
	})
}
