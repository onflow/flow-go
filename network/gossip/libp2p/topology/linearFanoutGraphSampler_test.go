package topology

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/utils/unittest"
)

type LinearFanoutGraphSamplerTestSuite struct {
	suite.Suite
	firstNodeSampler *LinearFanoutGraphSampler
	all              flow.IdentityList
}

// TestNewLinearFanoutGraphSamplerTestSuite runs all tests in this test suite.
func TestNewLinearFanoutGraphSamplerTestSuite(t *testing.T) {
	suite.Run(t, new(LinearFanoutGraphSamplerTestSuite))
}

// SetupTest is executed before any other test method in this test suite.
func (suite *LinearFanoutGraphSamplerTestSuite) SetupTest() {
	// creates 100 identities of all roles
	suite.all = unittest.IdentityListFixture(100, unittest.WithAllRoles())

	// creates a graph sampler for the first node in `all` list
	// we use this sampler for some isolated unit testing down the suite.
	sampler, err := NewLinearFanoutGraphSampler(suite.all[0].NodeID)
	require.NoError(suite.T(), err)
	suite.firstNodeSampler = sampler
}

// TestLinearFanout_UnconditionalSampling evaluates that sampling a connected graph fanout
// with an empty `shouldHave` list follows the LinearFanoutFunc,
// and it also does not contain duplicate element.
func (suite *LinearFanoutGraphSamplerTestSuite) TestLinearFanout_UnconditionalSampling() {
	// samples with no `shouldHave` set.
	sample, err := suite.firstNodeSampler.SampleConnectedGraph(suite.all, nil)
	require.NoError(suite.T(), err)

	// the LinearFanoutGraphSampler utilizes the LinearFanoutFunc. Hence any sample it makes should have
	// the size of greater than or equal to applying LinearFanoutFunc over the original set.
	expectedFanout := LinearFanoutFunc(len(suite.all))
	require.Equal(suite.T(), len(sample), expectedFanout)

	// checks sample does not include any duplicate
	suite.uniquenessCheck(sample)
}

// TestLinearFanout_ConditionalSampling evaluates that sampling a connected graph fanout with a shouldHave set
// follows the LinearFanoutFunc, and it also does not contain duplicate element.
func (suite *LinearFanoutGraphSamplerTestSuite) TestLinearFanout_ConditionalSampling() {
	// samples 10 ids into shouldHave and excludes them from all into others.
	shouldHave := suite.all.Sample(10)

	sample, err := suite.firstNodeSampler.SampleConnectedGraph(suite.all, shouldHave)
	require.NoError(suite.T(), err)

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

// TestLinearFanoutSmallerAll evaluates that sampling a connected graph fanout with a shouldHave set
// that is greater than `all` set in size, returns the `shouldHave` set.
func (suite *LinearFanoutGraphSamplerTestSuite) TestLinearFanoutSmallerAll() {
	// samples 10 ids into 'shouldHave'.
	shouldHave := suite.all.Sample(10)
	// samples a smaller component of all with 5 nodes and combines with `shouldHave`
	smallerAll := suite.all.Filter(filter.Not(filter.In(shouldHave))).Sample(5).Union(shouldHave)

	// total size of smallerAll is 15, and it requires a linear fanout of 8 which is less than
	// size of `shouldHave` set, so the shouldHave itself should return
	sample, err := suite.firstNodeSampler.SampleConnectedGraph(smallerAll, shouldHave)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), len(sample), len(shouldHave))
	require.ElementsMatch(suite.T(), sample, shouldHave)
}

// TestLinearFanoutNonSubsetShouldHave evaluates that trying to sample a connected graph when `shouldHave`
// is not a subset of `all` returns an error.
func (suite *LinearFanoutGraphSamplerTestSuite) TestLinearFanoutNonSubsetShouldHave() {
	// samples 10 ids into 'shouldHave',
	shouldHave := suite.all.Sample(10)
	// samples excludes one of the `shouldHave` ids from all, hence it is no longer a subset
	excludedAll := suite.all.Filter(filter.Not(filter.HasNodeID(shouldHave[0].NodeID)))

	// since `shouldHave` is not a subset of `excludedAll` it should return an error
	_, err := suite.firstNodeSampler.SampleConnectedGraph(excludedAll, shouldHave)
	require.Error(suite.T(), err)
}

// TestLinearFanoutNonSubsetEmptyAll evaluates that trying to sample a connected graph when `all`
// is empty returns an error.
func (suite *LinearFanoutGraphSamplerTestSuite) TestLinearFanoutNonSubsetEmptyAll() {
	// samples 10 ids into 'shouldHave'.
	shouldHave := suite.all.Sample(10)

	// sampling with empty all should return an error
	_, err := suite.firstNodeSampler.SampleConnectedGraph(flow.IdentityList{}, shouldHave)
	require.Error(suite.T(), err)

	// sampling with empty all should return an error
	_, err = suite.firstNodeSampler.SampleConnectedGraph(flow.IdentityList{}, nil)
	require.Error(suite.T(), err)

	// sampling with empty all should return an error
	_, err = suite.firstNodeSampler.SampleConnectedGraph(nil, nil)
	require.Error(suite.T(), err)
}

// TestConnectednessUnconditionally evaluates that samples returned by the LinearFanoutGraphSampler with
// empty `shouldHave` constitute a connected graph.
func (suite *LinearFanoutGraphSamplerTestSuite) TestConnectednessUnconditionally() {
	adjMap := make(map[flow.Identifier]flow.IdentityList)
	for _, id := range suite.all {
		// creates a graph sampler for the node
		graphSampler, err := NewLinearFanoutGraphSampler(id.NodeID)
		require.NoError(suite.T(), err)

		// samples a graph and stores it in adjacency map
		sample, err := graphSampler.SampleConnectedGraph(suite.all, nil)
		adjMap[id.NodeID] = sample
	}

	CheckGraphConnected(suite.T(), adjMap, suite.all, filter.In(suite.all))
}

// TestConnectednessWithNoShouldHave evaluates that samples returned by the LinearFanoutGraphSampler with
// some `shouldHave` constitute a connected graph.
func (suite *LinearFanoutGraphSamplerTestSuite) TestConnectednessConditionally() {
	adjMap := make(map[flow.Identifier]flow.IdentityList)
	for _, id := range suite.all {
		// creates a graph sampler for the node
		graphSampler, err := NewLinearFanoutGraphSampler(id.NodeID)
		require.NoError(suite.T(), err)

		// samples a graph and stores it in adjacency map
		// sampling is done with a non-empty should have subset of 10 randomly chosen ids
		shouldHave := suite.all.Sample(10)
		sample, err := graphSampler.SampleConnectedGraph(suite.all, shouldHave)

		// evaluates inclusion of should haves in sample
		for _, shouldHaveID := range shouldHave {
			require.Contains(suite.T(), sample, shouldHaveID)
		}
		adjMap[id.NodeID] = sample
	}

	CheckGraphConnected(suite.T(), adjMap, suite.all, filter.In(suite.all))
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
