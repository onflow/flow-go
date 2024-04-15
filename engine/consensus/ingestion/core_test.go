package ingestion

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	mockmempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestIngestionCore(t *testing.T) {
	suite.Run(t, new(IngestionCoreSuite))
}

type IngestionCoreSuite struct {
	suite.Suite

	accessID flow.Identifier
	collID   flow.Identifier
	conID    flow.Identifier
	execID   flow.Identifier
	verifID  flow.Identifier
	head     *flow.Header

	finalIdentities flow.IdentityList // identities at finalized state
	refIdentities   flow.IdentityList // identities at reference block state
	epochCounter    uint64            // epoch for the cluster originating the guarantee
	clusterMembers  flow.IdentityList // members of the cluster originating the guarantee
	clusterID       flow.ChainID      // chain ID of the cluster originating the guarantee

	final *mockprotocol.Snapshot // finalized state snapshot
	ref   *mockprotocol.Snapshot // state snapshot w.r.t. reference block

	query   *mockprotocol.EpochQuery
	epoch   *mockprotocol.Epoch
	headers *mockstorage.Headers
	pool    *mockmempool.Guarantees

	core *Core
}

func (suite *IngestionCoreSuite) SetupTest() {

	head := unittest.BlockHeaderFixture()
	head.Height = 2 * flow.DefaultTransactionExpiry

	access := unittest.IdentityFixture(unittest.WithRole(flow.RoleAccess))
	con := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	coll := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exec := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verif := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	suite.accessID = access.NodeID
	suite.conID = con.NodeID
	suite.collID = coll.NodeID
	suite.execID = exec.NodeID
	suite.verifID = verif.NodeID

	suite.epochCounter = 1
	suite.clusterMembers = flow.IdentityList{coll}
	suite.clusterID = cluster.CanonicalClusterID(suite.epochCounter, suite.clusterMembers.NodeIDs())

	identities := flow.IdentityList{access, con, coll, exec, verif}
	suite.finalIdentities = identities.Copy()
	suite.refIdentities = identities.Copy()

	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()
	state := &mockprotocol.State{}
	final := &mockprotocol.Snapshot{}
	ref := &mockprotocol.Snapshot{}
	suite.query = &mockprotocol.EpochQuery{}
	suite.epoch = &mockprotocol.Epoch{}
	headers := &mockstorage.Headers{}
	pool := &mockmempool.Guarantees{}
	cluster := &mockprotocol.Cluster{}

	// this state basically works like a normal protocol state
	// returning everything correctly, using the created header
	// as head of the protocol state
	state.On("Final").Return(final)
	final.On("Head").Return(head, nil)
	final.On("Identity", mock.Anything).Return(
		func(nodeID flow.Identifier) *flow.Identity {
			identity, _ := suite.finalIdentities.ByNodeID(nodeID)
			return identity
		},
		func(nodeID flow.Identifier) error {
			_, ok := suite.finalIdentities.ByNodeID(nodeID)
			if !ok {
				return protocol.IdentityNotFoundError{NodeID: nodeID}
			}
			return nil
		},
	)
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			return suite.finalIdentities.Filter(selector)
		},
		nil,
	)
	ref.On("Epochs").Return(suite.query)
	suite.query.On("Current").Return(suite.epoch)
	cluster.On("Members").Return(suite.clusterMembers.ToSkeleton())
	suite.epoch.On("ClusterByChainID", mock.Anything).Return(
		func(chainID flow.ChainID) protocol.Cluster {
			if chainID == suite.clusterID {
				return cluster
			}
			return nil
		},
		func(chainID flow.ChainID) error {
			if chainID == suite.clusterID {
				return nil
			}
			return protocol.ErrClusterNotFound
		})

	state.On("AtBlockID", mock.Anything).Return(ref)
	ref.On("Identity", mock.Anything).Return(
		func(nodeID flow.Identifier) *flow.Identity {
			identity, _ := suite.refIdentities.ByNodeID(nodeID)
			return identity
		},
		func(nodeID flow.Identifier) error {
			_, ok := suite.refIdentities.ByNodeID(nodeID)
			if !ok {
				return protocol.IdentityNotFoundError{NodeID: nodeID}
			}
			return nil
		},
	)

	// we need to return the head as it's also used as reference block
	headers.On("ByBlockID", head.ID()).Return(head, nil)

	// only used for metrics, nobody cares
	pool.On("Size").Return(uint(0))

	ingest := NewCore(unittest.Logger(), tracer, metrics, state, headers, pool)

	suite.head = head
	suite.final = final
	suite.ref = ref
	suite.headers = headers
	suite.pool = pool
	suite.core = ingest
}

func (suite *IngestionCoreSuite) TestOnGuaranteeNewFromCollection() {

	guarantee := suite.validGuarantee()

	// the guarantee is not part of the memory pool yet
	suite.pool.On("Has", guarantee.ID()).Return(false)
	suite.pool.On("Add", guarantee).Return(true)

	// submit the guarantee as if it was sent by a collection node
	err := suite.core.OnGuarantee(suite.collID, guarantee)
	suite.Assert().NoError(err, "should not error on new guarantee from collection node")

	// check that the guarantee has been added to the mempool
	suite.pool.AssertCalled(suite.T(), "Add", guarantee)

}

func (suite *IngestionCoreSuite) TestOnGuaranteeOld() {

	guarantee := suite.validGuarantee()

	// the guarantee is part of the memory pool
	suite.pool.On("Has", guarantee.ID()).Return(true)
	suite.pool.On("Add", guarantee).Return(true)

	// submit the guarantee as if it was sent by a collection node
	err := suite.core.OnGuarantee(suite.collID, guarantee)
	suite.Assert().NoError(err, "should not error on old guarantee")

	// check that the guarantee has _not_ been added to the mempool
	suite.pool.AssertNotCalled(suite.T(), "Add", guarantee)

}

func (suite *IngestionCoreSuite) TestOnGuaranteeNotAdded() {

	guarantee := suite.validGuarantee()

	// the guarantee is not already part of the memory pool
	suite.pool.On("Has", guarantee.ID()).Return(false)
	suite.pool.On("Add", guarantee).Return(false)

	// submit the guarantee as if it was sent by a collection node
	err := suite.core.OnGuarantee(suite.collID, guarantee)
	suite.Assert().NoError(err, "should not error when guarantee was already added")

	// check that the guarantee has been added to the mempool
	suite.pool.AssertCalled(suite.T(), "Add", guarantee)

}

// TestOnGuaranteeNoGuarantors tests that a collection without any guarantors is rejected.
// We expect an engine.InvalidInputError.
func (suite *IngestionCoreSuite) TestOnGuaranteeNoGuarantors() {
	// create a guarantee without any signers
	guarantee := suite.validGuarantee()
	guarantee.SignerIndices = nil

	// the guarantee is not part of the memory pool
	suite.pool.On("Has", guarantee.ID()).Return(false)
	suite.pool.On("Add", guarantee).Return(true)

	// submit the guarantee as if it was sent by a consensus node
	err := suite.core.OnGuarantee(suite.collID, guarantee)
	suite.Assert().Error(err, "should error with missing guarantor")
	suite.Assert().True(engine.IsInvalidInputError(err))

	// check that the guarantee has _not_ been added to the mempool
	suite.pool.AssertNotCalled(suite.T(), "Add", guarantee)
}

func (suite *IngestionCoreSuite) TestOnGuaranteeExpired() {

	// create an alternative block
	header := unittest.BlockHeaderFixture()
	header.Height = suite.head.Height - flow.DefaultTransactionExpiry - 1
	suite.headers.On("ByBlockID", header.ID()).Return(header, nil)

	// create a guarantee signed by the collection node and referencing the
	// current head of the protocol state
	guarantee := suite.validGuarantee()
	guarantee.ReferenceBlockID = header.ID()

	// the guarantee is not part of the memory pool
	suite.pool.On("Has", guarantee.ID()).Return(false)
	suite.pool.On("Add", guarantee).Return(true)

	// submit the guarantee as if it was sent by a consensus node
	err := suite.core.OnGuarantee(suite.collID, guarantee)
	suite.Assert().Error(err, "should error with expired collection")
	suite.Assert().True(engine.IsOutdatedInputError(err))
}

// TestOnGuaranteeReferenceBlockFromWrongEpoch validates that guarantees which contain a ChainID
// that is inconsistent with the reference block (ie. the ChainID either refers to a non-existent
// cluster, or a cluster for a different epoch) should be considered invalid inputs.
func (suite *IngestionCoreSuite) TestOnGuaranteeReferenceBlockFromWrongEpoch() {
	// create a guarantee from a cluster in a different epoch
	guarantee := suite.validGuarantee()
	guarantee.ChainID = cluster.CanonicalClusterID(suite.epochCounter+1, suite.clusterMembers.NodeIDs())

	// the guarantee is not part of the memory pool
	suite.pool.On("Has", guarantee.ID()).Return(false)

	// submit the guarantee as if it was sent by a collection node
	err := suite.core.OnGuarantee(suite.collID, guarantee)
	suite.Assert().Error(err, "should error with expired collection")
	suite.Assert().True(engine.IsInvalidInputError(err))
}

// TestOnGuaranteeInvalidGuarantor verifiers that collections with any _unknown_
// signer are rejected.
func (suite *IngestionCoreSuite) TestOnGuaranteeInvalidGuarantor() {

	// create a guarantee  and add random (unknown) signer ID
	guarantee := suite.validGuarantee()
	guarantee.SignerIndices = []byte{4}

	// the guarantee is not part of the memory pool
	suite.pool.On("Has", guarantee.ID()).Return(false)
	suite.pool.On("Add", guarantee).Return(true)

	// submit the guarantee as if it was sent by a collection node
	err := suite.core.OnGuarantee(suite.collID, guarantee)
	suite.Assert().Error(err, "should error with invalid guarantor")
	suite.Assert().True(engine.IsInvalidInputError(err), err)
	suite.Assert().True(signature.IsInvalidSignerIndicesError(err), err)

	// check that the guarantee has _not_ been added to the mempool
	suite.pool.AssertNotCalled(suite.T(), "Add", guarantee)
}

// test that just after an epoch boundary we still accept guarantees from collectors
// in clusters from the previous epoch (and collectors which are leaving the network
// at this epoch boundary).
func (suite *IngestionCoreSuite) TestOnGuaranteeEpochEnd() {

	// The finalized state contains the identity of a collector that:
	//  * was active in the previous epoch but is leaving as of the current epoch
	//  * wasn't ejected and has positive initial weight
	// This happens when we finalize the final block of the epoch during
	// which this node requested to unstake
	colID, ok := suite.finalIdentities.ByNodeID(suite.collID)
	suite.Require().True(ok)
	colID.EpochParticipationStatus = flow.EpochParticipationStatusLeaving

	guarantee := suite.validGuarantee()

	// the guarantee is not part of the memory pool
	suite.pool.On("Has", guarantee.ID()).Return(false)
	suite.pool.On("Add", guarantee).Return(true).Once()

	// submit the guarantee as if it was sent by the collection node which
	// is leaving at the current epoch boundary
	err := suite.core.OnGuarantee(suite.collID, guarantee)
	suite.Assert().NoError(err, "should not error with collector from ending epoch")

	// check that the guarantee has been added to the mempool
	suite.pool.AssertExpectations(suite.T())
}

func (suite *IngestionCoreSuite) TestOnGuaranteeUnknownOrigin() {

	guarantee := suite.validGuarantee()

	// the guarantee is not part of the memory pool
	suite.pool.On("Has", guarantee.ID()).Return(false)
	suite.pool.On("Add", guarantee).Return(true)

	// submit the guarantee with an unknown origin
	err := suite.core.OnGuarantee(unittest.IdentifierFixture(), guarantee)
	suite.Assert().Error(err)
	suite.Assert().True(engine.IsInvalidInputError(err))

	suite.pool.AssertNotCalled(suite.T(), "Add", guarantee)

}

// validGuarantee returns a valid collection guarantee based on the suite state.
func (suite *IngestionCoreSuite) validGuarantee() *flow.CollectionGuarantee {
	guarantee := unittest.CollectionGuaranteeFixture()
	guarantee.ChainID = suite.clusterID

	signerIndices, err := signature.EncodeSignersToIndices(
		[]flow.Identifier{suite.collID}, []flow.Identifier{suite.collID})
	require.NoError(suite.T(), err)

	guarantee.SignerIndices = signerIndices
	guarantee.ReferenceBlockID = suite.head.ID()
	return guarantee
}
