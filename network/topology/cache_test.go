package topology

import (
	"fmt"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/mocknetwork"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestCache_GenerateFanout_HappyPath evaluates some weak caching guarantees over Cache with a mock underlying topology.
// The guarantees include calling the underlying topology only once as long as the input is the same, caching the result,
// and returning the same result over consecutive invocations with the same input.
// The evaluations are weak as they go through a mock topology.
func TestCache_GenerateFanout_HappyPath(t *testing.T) {
	// mocks underlying topology
	top := &mocknetwork.Topology{}
	log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
	ids := unittest.IdentityListFixture(100)
	fanout := ids.Sample(10)
	top.On("GenerateFanout", ids).Return(fanout, nil).Once()

	cache := NewCache(log, top)

	// Testing cache resolving
	//
	// Over consecutive invocations of cache with the same input, the same output should be returned.
	// The output should be resolved by the cache without asking the underlying topology more than once.
	prevFanout := fanout
	for i := 0; i < 100; i++ {
		newFanout, err := cache.GenerateFanout(ids, nil)
		require.NoError(t, err)

		require.Equal(t, prevFanout, newFanout)
		prevFanout = newFanout
	}

	// underlying topology should be called only once through all consecutive calls (with same input).
	mock.AssertExpectationsForObjects(t, top)
}

// TestCache_GenerateFanout_Error evaluates that returning error by underlying topology on GenerateFanout, results in
// cache to invalidate itself.
func TestCache_GenerateFanout_Error(t *testing.T) {
	// mocks underlying topology returning a fanout
	top := &mocknetwork.Topology{}
	log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
	ids := unittest.IdentityListFixture(100)
	top.On("GenerateFanout", ids).Return(nil, fmt.Errorf("fanout-generation-error")).Once()

	// assumes cache holding some fanout
	cache := NewCache(log, top)
	cache.cachedFanout = ids.Sample(10)
	cache.idsFP = unittest.IdentifierFixture()

	// returning error on fanout generation should invalidate cache
	// same error should be returned.
	fanout, err := cache.GenerateFanout(ids, nil)
	require.Error(t, err)
	require.Nil(t, fanout)
	require.Equal(t, cache.idsFP, flow.Identifier{})
	require.Empty(t, cache.cachedFanout)

	// underlying topology should be called only once.
	mock.AssertExpectationsForObjects(t, top)
}

// TestCache_InputChange_IDs evaluates that if identity list input to GenerateFanout changes,
// the cache is invalidated and updated.
func TestCache_InputChange_IDs(t *testing.T) {
	// mocks underlying topology returning a fanout
	top := &mocknetwork.Topology{}
	ids := unittest.IdentityListFixture(100)
	fanout := ids.Sample(10)
	channels := network.ChannelList{"channel1", "channel2"}
	top.On("GenerateFanout", mock.Anything, channels).Return(fanout, nil).Once()

	// assumes cache holding some fanout for the same
	// ids and channels defined above.
	cache := NewCache(zerolog.Nop(), top)
	cache.cachedFanout = ids.Sample(10)
	cache.idsFP = ids.Fingerprint()
	cache.chansFP = channels.ID()

	// cache content should change once input ids list to GenerateFanout changes.
	// drops last id in the list to imitate a change.
	newFanout, err := cache.GenerateFanout(ids[:len(ids)-1], channels)
	require.NoError(t, err)
	require.Equal(t, cache.idsFP, ids[:len(ids)-1].Fingerprint())
	// channels input did not change, hence channels fingerprint should not be changed.
	require.Equal(t, cache.chansFP, channels.ID())
	// returned (new) fanout should be equal to the mocked fanout, and
	// also should be cached.
	require.Equal(t, cache.cachedFanout, fanout)
	require.Equal(t, cache.cachedFanout, newFanout)

	// underlying topology should be called only once.
	mock.AssertExpectationsForObjects(t, top)
}

//// TestCache_InputChange_Channels evaluates that if identity list input to GenerateFanout changes,
//// the cache is invalidated and updated.
//func TestCache_InputChange_Channels(t *testing.T) {
//	// mocks underlying topology returning a fanout
//	top := &mocknetwork.Topology{}
//	log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
//	ids := unittest.IdentityListFixture(100)
//	fanout := ids.Sample(10)
//	channels := network.ChannelList{"channel1", "channel2"}
//	newChannels := append(channel, "channel3")
//	top.On("GenerateFanout", ids, channels).Return(fanout, nil).Once()
//
//	// assumes cache holding some fanout
//	cache := NewCache(log, top)
//	cache.cachedFanout = ids.Sample(10)
//	cache.idsFP = unittest.IdentifierFixture()
//
//	// cache content should change once input channels to GenerateFanout changes.
//	newFanout, err := cache.GenerateFanout(ids, newChannels)
//	require.NoError(t, err)
//	// ids fingerprint in the cache should not change, since the original input did not change.
//	require.Equal(t, cache.idsFP, ids.Fingerprint())
//	// channels input changed, hence channels fingerprint should be changed.
//	require.Equal(t, cache.chansFP, newChannels.ID())
//	// returned (new) fanout should be equal to the mocked fanout, and
//	// also should be cached.
//	require.Equal(t, cache.cachedFanout, fanout)
//	require.Equal(t, cache.cachedFanout, newFanout)
//
//	// underlying topology should be called only once.
//	mock.AssertExpectationsForObjects(t, top)
//}

// TestCache_TopicBased evaluates strong cache guarantees over an underlying TopicBased cache. The guarantees
// include a deterministic fanout as long as the input is the same, updating the cache once input gets changed, and
// retaining on that.
func TestCache_TopicBased(t *testing.T) {
	// Creates a topology cache for a verification node based on its TopicBased topology.
	ids := unittest.IdentityListFixture(100, unittest.WithAllRoles())
	myId := ids.Filter(filter.HasRole(flow.RoleVerification))[0]
	subManagers := MockSubscriptionManager(t, flow.IdentityList{myId})
	channels := subManagers[0].Channels()

	top, err := NewTopicBasedTopology(myId.NodeID, zerolog.Nop(), &mockprotocol.State{})
	require.NoError(t, err)
	cache := NewCache(zerolog.Nop(), top)

	// Testing deterministic behavior
	//
	// Over consecutive invocations of cache with the same input, the same output should be returned.
	prevFanout, err := cache.GenerateFanout(ids, channels)
	require.NoError(t, err)
	require.NotEmpty(t, prevFanout)
	// requires same fanout as long as the input is the same.
	requireDeterministicBehavior(t, cache, prevFanout, ids, channels)

	// Testing cache invalidation and update by identity list change
	//
	// Evicts one identity from ids list and cache should be invalidated and updated with a new fanout.
	ids = ids[:len(ids)-1]
	newFanout, err := cache.GenerateFanout(ids, channels)
	require.NoError(t, err)
	require.NotEmpty(t, newFanout)
	require.NotEqual(t, newFanout, prevFanout)
	// requires same fanout as long as the input is the same.
	requireDeterministicBehavior(t, cache, newFanout, ids, channels)

	// Testing cache invalidation and update by identity channel list change
	//
	// Keeps the same identity list but removes all channels except one from the input channel list.
	// Cache should be invalidated and updated with a new fanout.
	prevFanout = newFanout.Copy()
	channels = channels[:1]
	newFanout, err = cache.GenerateFanout(ids, channels)
	require.NoError(t, err)
	require.NotEmpty(t, newFanout)
	require.NotEqual(t, newFanout, prevFanout)
	// requires same fanout as long as the input is the same.
	requireDeterministicBehavior(t, cache, newFanout, ids, channels)

}

// requireDeterministicBehavior evaluates that consecutive invocations of cache on fanout generation with the same input (i.e., ids and channels),
// results in the same output fanout as the one passed to the method.
func requireDeterministicBehavior(t *testing.T, cache *Cache, fanout flow.IdentityList, ids flow.IdentityList, channels network.ChannelList) {
	// Testing deterministic
	//
	// Over consecutive invocations of cache with the same (new) input, the same
	// (new) output should be returned.
	for i := 0; i < 100; i++ {
		newFanout, err := cache.GenerateFanout(ids, channels)
		require.NoError(t, err)
		require.NotEmpty(t, newFanout)

		require.Equal(t, fanout, newFanout)
	}
}
