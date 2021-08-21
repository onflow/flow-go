package p2p

import (
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gotest.tools/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type HierarchicalTranslatorTestSuite struct {
	suite.Suite
	translator *HierarchicalIDTranslator
	ids        flow.IdentityList
}

func (suite *HierarchicalTranslatorTestSuite) SetupTest() {
	suite.ids = unittest.IdentityListFixture(2, unittest.WithKeys)
	t1, err := NewFixedTableIdentityTranslator(suite.ids[:1])
	require.NoError(suite.T(), err)
	t2, err := NewFixedTableIdentityTranslator(suite.ids[1:])
	require.NoError(suite.T(), err)

	suite.translator = NewHierarchicalIDTranslator(t1, t2)
}

func TestHierarchicalTranslator(t *testing.T) {
	suite.Run(t, new(HierarchicalTranslatorTestSuite))
}

func (suite *HierarchicalTranslatorTestSuite) TestFirstTranslatorSuccess() {
	pid, err := suite.translator.GetPeerID(suite.ids[0].NodeID)
	require.NoError(suite.T(), err)
	fid, err := suite.translator.GetFlowID(pid)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), fid, suite.ids[0].NodeID)
}

func (suite *HierarchicalTranslatorTestSuite) TestSecondTranslatorSuccess() {
	pid, err := suite.translator.GetPeerID(suite.ids[1].NodeID)
	require.NoError(suite.T(), err)
	fid, err := suite.translator.GetFlowID(pid)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), fid, suite.ids[1].NodeID)
}

func (suite *HierarchicalTranslatorTestSuite) TestTranslationFailure() {
	fid := unittest.IdentifierFixture()
	_, err := suite.translator.GetPeerID(fid)
	require.Error(suite.T(), err)

	key, err := LibP2PPrivKeyFromFlow(generateNetworkingKey(suite.T()))
	require.NoError(suite.T(), err)
	pid, err := peer.IDFromPrivateKey(key)
	require.NoError(suite.T(), err)
	_, err = suite.translator.GetFlowID(pid)
	require.Error(suite.T(), err)
}
