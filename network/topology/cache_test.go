package topology

import (
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

// TestCache_GenerateFanout evaluates some weak caching guarantees over Cache with a mock underlying topology.
func TestCache_GenerateFanout(t *testing.T) {
	top := &mocknetwork.Topology{}
	log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)

	ids := unittest.IdentityListFixture(100)
	fanout := ids.Sample(10)

	// mock underlying topology should be called only once.
	top.On("GenerateFanout", ids).Return(fanout, nil).Once()

	cache := NewCache(log, top)

	// over 100 invocations of cache with the same input, the same
	// output should be returned.
	// The output should be resolved by the cache
	// without asking the underlying topology more than once.
	prevFanout := fanout
	for i := 0; i < 100; i++ {
		newFanout, err := cache.GenerateFanout(ids)
		require.NoError(t, err)

		require.Equal(t, prevFanout, newFanout)
		prevFanout = newFanout
	}

	// underlying topology should be called only once.
	mock.AssertExpectationsForObjects(t, top)
}

// TestCache_TopicBased
func TestCache_TopicBased(t *testing.T) {
	// creates a topology cache for a verification node based on its
	// TopicBased topology.
	log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
	ids := unittest.IdentityListFixture(100, unittest.WithAllRoles())
	myId := ids.Filter(filter.HasRole(flow.RoleVerification))[0]
	mySubManager := MockSubscriptionManager(t, flow.IdentityList{myId})

	top, err := NewTopicBasedTopology(myId.NodeID, log, &mockprotocol.State{}, mySubManager[0])
	require.NoError(t, err)

	cache := NewCache(log, top)

	// Testing deterministic behavior
	//
	// over 100 invocations of cache with the same input, the same
	// output should be returned.
	prevFanout, err := cache.GenerateFanout(ids)
	require.NoError(t, err)
	for i := 0; i < 100; i++ {
		newFanout, err := cache.GenerateFanout(ids)
		require.NoError(t, err)

		require.Equal(t, prevFanout, newFanout)
		prevFanout = newFanout
	}
}
