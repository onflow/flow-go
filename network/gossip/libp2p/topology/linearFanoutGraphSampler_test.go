package topology

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type LinearFanoutGraphSamplerTestSuite struct {
	suite.Suite
	sampler *LinearFanoutGraphSampler
	all     flow.IdentityList
}

// TestNewLinearFanoutGraphSamplerTestSuite runs all tests in this test suite.
func TestNewLinearFanoutGraphSamplerTestSuite(t *testing.T) {
	suite.Run(t, new(LinearFanoutGraphSamplerTestSuite))
}

// SetupTest is executed before any other test method in this test suite.
func (suite *LinearFanoutGraphSamplerTestSuite) SetupTest() {
	suite.all = unittest.IdentityListFixture(100, unittest.WithAllRoles())

	sampler, err := NewLinearFanoutGraphSampler(suite.all[0].NodeID)
	require.NoError(suite.T(), err)

	suite.sampler = sampler
}

// TestLinearFanoutNoShouldHave evaluates that sampling a connected graph fanout
// follows the LinearFanoutFunc, and it also does not contain duplicate element.
func (suite *LinearFanoutGraphSamplerTestSuite) TestLinearFanoutNoShouldHave() {
	sample := suite.sampler.SampleConnectedGraph(suite.all, nil)

	// the LinearFanoutGraphSampler utilizes the LinearFanoutFunc. Hence any sample it makes should have
	// the size of greater than or equal to applying LinearFanoutFunc over the original set.
	expectedFanout := LinearFanoutFunc(len(suite.all))
	require.Equal(suite.T(), len(sample), expectedFanout)

	// checks sample does not include any duplicate
	suite.uniquenessCheck(sample)
}

// TestLinearFanoutNoShouldHave evaluates that sampling a connected graph fanout with a shouldHave set
// follows the LinearFanoutFunc, and it also does not contain duplicate element.
func (suite *LinearFanoutGraphSamplerTestSuite) TestLinearFanoutWithShouldHave() {
	// samples 10 ids into shouldHave and excludes them from all into others.
	shouldHave := suite.all.Sample(10)

	sample := suite.sampler.SampleConnectedGraph(suite.all, shouldHave)

	// the LinearFanoutGraphSampler utilizes the LinearFanoutFunc. Hence any sample it makes should have
	// the size of greater than or equal to applying LinearFanoutFunc over the original set.
	expectedFanout := LinearFanoutFunc(len(suite.all))
	require.Equal(suite.T(), len(sample), expectedFanout)

	// checks sample does not include any duplicate
	suite.uniquenessCheck(sample)

	// checks inclusion of all shouldHave ones into sample
	for _, id := range shouldHave {
		require.Contains(suite.T(), sample, id)
	}
}

// uniquenessCheck is a test helper method that fails the test if ids include any duplicate identity.
func (suite *LinearFanoutGraphSamplerTestSuite) uniquenessCheck(ids flow.IdentityList) {
	seen := make(map[flow.Identity]struct{})
	for _, id := range ids {
		// checks if id is duplicate in ids list
		_, ok := seen[*id]
		require.False(suite.T(), ok)

		// marks id as seen
		seen[*id] = struct{}{}
	}
}
