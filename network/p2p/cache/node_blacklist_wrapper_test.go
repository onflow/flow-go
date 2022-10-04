package cache_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/module/id"
	mocks "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/p2p/cache"
	"github.com/onflow/flow-go/utils/unittest"
)

type NodeBlacklistWrapperTestSuite struct {
	suite.Suite
	DB       *badger.DB
	provider id.IdentityProvider
	wrapper  *cache.NodeBlacklistWrapper
}

func (s *NodeBlacklistWrapperTestSuite) SetupTest() {
	s.DB, _ = unittest.TempBadgerDB(s.T())
	s.provider = new(mocks.IdentityProvider)

	var err error
	s.wrapper, err = cache.NewNodeBlacklistWrapper(s.provider, s.DB)
	require.NoError(s.T(), err)
}

func TestNodeBlacklistWrapperTestSuite(t *testing.T) {
	suite.Run(t, new(NodeBlacklistWrapperTestSuite))
}

// TestHonestNode verifies that the wrapper forwards the identities
// from the wrapped `IdentityProvider` without modification.
func (s *NodeBlacklistWrapperTestSuite) TestHonestNode() {
	id := unittest.IdentifierFixture()
	identity, found := s.wrapper.ByNodeID(id)
	require.False(s.T(), found)
	require.Nil(s.T(), identity)
}

// TestUnknownNode verifies that the wrapper forwards nil identities
// irrespective of the boolean return values.
func (s *NodeBlacklistWrapperTestSuite) TestUnknownNode() {
	id := unittest.IdentifierFixture()
	identity, found := s.wrapper.ByNodeID(id)
	require.False(s.T(), found)
	require.Nil(s.T(), identity)
}
