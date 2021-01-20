package topology

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/network/mocknetwork"
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
		newFanout = prevFanout
	}

	// underlying topology should be called only once.
	mock.AssertExpectationsForObjects(t, top)
}
