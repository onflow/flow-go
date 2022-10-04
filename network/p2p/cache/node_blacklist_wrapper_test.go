package cache_test

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/network/p2p/cache"
	"github.com/onflow/flow-go/utils/unittest"
)

type NodeBlacklistWrapperTestSuite struct {
	suite.Suite
	DB       *badger.DB
	provider *cache.ProtocolStateIDCache
}

func (s *NodeBlacklistWrapperTestSuite) SetupTest() {
	s.DB, _ = unittest.TempBadgerDB(s.T())
}
