package follower

import (
	"github.com/onflow/flow-go/engine/common/follower/mock"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCache(t *testing.T) {
	suite.Run(t, new(CacheSuite))
}

type CacheSuite struct {
	suite.Suite

	onEquivocation *mock.OnEquivocation
	cache          *Cache
}

func (s *CacheSuite) SetupTest() {
	collector := metrics.NewNoopCollector()
	s.onEquivocation = mock.NewOnEquivocation(s.T())
	s.cache = NewCache(unittest.Logger(), 1000, collector, s.onEquivocation.Execute)
}

// TestPeek tests if previously added block can be queried by block ID
func (s *CacheSuite) TestPeek() {
	block := unittest.BlockFixture()
	s.cache.AddBlocks([]*flow.Block{&block})
	actual := s.cache.Peek(block.ID())
	require.NotNil(s.T(), actual)
	require.Equal(s.T(), actual.ID(), block.ID())
}
