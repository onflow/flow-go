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

// TestCache_Update evaluates that if input to GenerateFanout changes, the cache is invalidated and updated.
func TestCache_Update(t *testing.T) {
	// mocks underlying topology returning a fanout
	top := &mocknetwork.Topology{}
	log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
	ids := unittest.IdentityListFixture(100)
	fanout := ids.Sample(10)
	top.On("GenerateFanout", ids).Return(fanout, nil).Once()

	// assumes cache holding some fanout
	cache := NewCache(log, top)
	cache.cachedFanout = ids.Sample(10)
	cache.idsFP = unittest.IdentifierFixture()

	// cache content should change once idsFP to GenerateFanout changes.
	newFanout, err := cache.GenerateFanout(ids, nil)
	require.NoError(t, err)
	require.Equal(t, cache.idsFP, ids.Fingerprint())
	require.Equal(t, cache.cachedFanout, fanout)
	require.Equal(t, cache.cachedFanout, newFanout)

	// underlying topology should be called only once.
	mock.AssertExpectationsForObjects(t, top)
}

// TestCache_TopicBased evaluates strong cache guarantees over an underlying TopicBased cache. The guarantees
// include a deterministic fanout as long as the input is the same, updating the cache once input gets changed, and
// retaining on that.
func TestCache_TopicBased(t *testing.T) {
	// Creates a topology cache for a verification node based on its TopicBased topology.
	log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
	ids := unittest.IdentityListFixture(100, unittest.WithAllRoles())
	myId := ids.Filter(filter.HasRole(flow.RoleVerification))[0]
	subManagers := MockSubscriptionManager(t, flow.IdentityList{myId})

	top, err := NewTopicBasedTopology(myId.NodeID, log, &mockprotocol.State{})
	require.NoError(t, err)

	cache := NewCache(log, top)

	// Testing deterministic behavior
	//
	// Over consecutive invocations of cache with the same input, the same output should be returned.
	prevFanout, err := cache.GenerateFanout(ids, subManagers[0].Channels())
	require.NoError(t, err)
	for i := 0; i < 100; i++ {
		newFanout, err := cache.GenerateFanout(ids, nil)
		require.NoError(t, err)

		require.Equal(t, prevFanout, newFanout)
		prevFanout = newFanout
	}

	// Testing cache invalidation and update
	//
	// Evicts one identity from ids list and cache should be invalidated and updated with a new fanout.
	ids = ids[:len(ids)-1]
	newFanout, err := cache.GenerateFanout(ids, nil)
	require.NoError(t, err)
	require.NotEqual(t, newFanout, prevFanout)

	// Testing deterministic behavior after an update
	//
	// After a cache update, over consecutive invocations of cache with the same (new) input, the same
	// (new) output should be returned.
	prevFanout = newFanout
	for i := 0; i < 100; i++ {
		newFanout, err := cache.GenerateFanout(ids, nil)
		require.NoError(t, err)

		require.Equal(t, prevFanout, newFanout)
		prevFanout = newFanout
	}
}
