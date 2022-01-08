package committees

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/indices"
	clusterstate "github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// ClusterSuite defines a test suite for the cluster committee.
type ClusterSuite struct {
	suite.Suite

	state    *protocolmock.State
	snap     *protocolmock.Snapshot
	cluster  *protocolmock.Cluster
	epoch    *protocolmock.Epoch
	payloads *storagemock.ClusterPayloads

	members flow.IdentityList
	root    *cluster.Block
	me      *flow.Identity

	com *Cluster
}

func TestClusterCommittee(t *testing.T) {
	suite.Run(t, new(ClusterSuite))
}

func (suite *ClusterSuite) SetupTest() {

	suite.state = new(protocolmock.State)
	suite.snap = new(protocolmock.Snapshot)
	suite.cluster = new(protocolmock.Cluster)
	suite.epoch = new(protocolmock.Epoch)
	suite.payloads = new(storagemock.ClusterPayloads)

	suite.members = unittest.IdentityListFixture(5, unittest.WithRole(flow.RoleCollection))
	suite.me = suite.members[0]
	counter := uint64(1)
	suite.root = clusterstate.CanonicalRootBlock(counter, suite.members)

	suite.cluster.On("EpochCounter").Return(counter)
	suite.cluster.On("Index").Return(uint(1))
	suite.cluster.On("Members").Return(suite.members)
	suite.cluster.On("RootBlock").Return(suite.root)
	suite.epoch.On("Counter").Return(counter, nil)
	inds := indices.ProtocolCollectorClusterLeaderSelection(suite.cluster.Index())
	suite.epoch.On("Seed", inds[0], inds[1], inds[2]).Return(unittest.SeedFixture(32), nil)

	var err error
	suite.com, err = NewClusterCommittee(
		suite.state,
		suite.payloads,
		suite.cluster,
		suite.epoch,
		suite.me.NodeID,
	)
	suite.Require().NoError(err)
}

// TestInvalidSigner tests that the InvalidSignerError sentinel is
// returned under the appropriate conditions.
func (suite *ClusterSuite) TestInvalidSigner() {

	// hook up cluster->main chain connection for root and non-root cluster block
	nonRootBlockID := unittest.IdentifierFixture()
	rootBlockID := suite.root.ID()

	refID := unittest.IdentifierFixture()            // reference block on main chain
	payload := cluster.EmptyPayload(refID)           // payload referencing main chain
	rootPayload := cluster.EmptyPayload(flow.ZeroID) // root cluster block payload

	suite.payloads.On("ByBlockID", nonRootBlockID).Return(&payload, nil)
	suite.payloads.On("ByBlockID", rootBlockID).Return(&rootPayload, nil)

	realIdentity := suite.members[1]
	realNonCommitteeIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	fakeID := unittest.IdentifierFixture()

	suite.state.On("AtBlockID", refID).Return(suite.snap)
	suite.snap.On("Identity", realIdentity.NodeID).Return(realIdentity, nil)
	suite.snap.On("Identity", realNonCommitteeIdentity.NodeID).Return(realNonCommitteeIdentity, nil)
	suite.snap.On("Identity", fakeID).Return(nil, protocol.IdentityNotFoundError{})

	suite.Run("should return InvalidSignerError for non-existent signer", func() {
		suite.Run("root block", func() {
			_, err := suite.com.Identity(rootBlockID, fakeID)
			suite.Assert().True(model.IsInvalidSignerError(err))
		})
		suite.Run("non-root block", func() {
			_, err := suite.com.Identity(nonRootBlockID, fakeID)
			suite.Assert().True(model.IsInvalidSignerError(err))
		})
	})

	suite.Run("should return InvalidSignerError for existent non-cluster-member", func() {
		suite.Run("root block", func() {
			_, err := suite.com.Identity(rootBlockID, realNonCommitteeIdentity.NodeID)
			suite.Assert().True(model.IsInvalidSignerError(err))
		})
		suite.Run("non-root block", func() {
			_, err := suite.com.Identity(nonRootBlockID, realNonCommitteeIdentity.NodeID)
			suite.Assert().True(model.IsInvalidSignerError(err))
		})
	})

	suite.Run("should return identity for existent cluster member", func() {
		suite.Run("root block", func() {
			actual, err := suite.com.Identity(rootBlockID, realIdentity.NodeID)
			suite.Require().NoError(err)
			suite.Assert().Equal(realIdentity, actual)
		})
		suite.Run("non-root block", func() {
			actual, err := suite.com.Identity(nonRootBlockID, realIdentity.NodeID)
			suite.Require().NoError(err)
			suite.Assert().Equal(realIdentity, actual)
		})
	})
}
