// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	mockmempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
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

	con1ID flow.Identifier
	con2ID flow.Identifier
	con3ID flow.Identifier
	collID flow.Identifier
	execID flow.Identifier
	head   *flow.Header

	finalIdentities flow.IdentityList // identities at finalized state
	refIdentities   flow.IdentityList // identities at reference block state

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

	con1 := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	con2 := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	con3 := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	coll := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exec := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	suite.con1ID = con1.NodeID
	suite.con2ID = con2.NodeID
	suite.con3ID = con3.NodeID
	suite.collID = coll.NodeID
	suite.execID = exec.NodeID

	clusters := flow.ClusterList{flow.IdentityList{coll}}

	identities := flow.IdentityList{con1, con2, con3, coll, exec}
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
	me := &mockmodule.Local{}
	pool := &mockmempool.Guarantees{}

	// this state basically works like a normal protocol state
	// returning everything correctly, using the created header
	// as head of the protocol state
	state.On("Final").Return(final)
	final.On("Head").Return(&head, nil)
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
		func(selector flow.IdentityFilter) flow.IdentityList {
			return suite.finalIdentities.Filter(selector)
		},
		nil,
	)
	ref.On("Epochs").Return(suite.query)
	suite.query.On("Current").Return(suite.epoch)
	suite.epoch.On("Clustering").Return(clusters, nil)

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

	// we use the first consensus node as our local identity
	me.On("NodeID").Return(suite.con1ID)
	me.On("NotMeFilter").Return(filter.Not(filter.HasNodeID(suite.con1ID)))

	// we need to return the head as it's also used as reference block
	headers.On("ByBlockID", head.ID()).Return(&head, nil)

	// only used for metrics, nobody cares
	pool.On("Size").Return(uint(0))

	ingest := &Core{
		tracer:  tracer,
		mempool: metrics,
		state:   state,
		headers: headers,
		me:      me,
		pool:    pool,
	}

	suite.head = &head
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

func (suite *IngestionCoreSuite) TestOnGuaranteeUnstaked() {

	guarantee := suite.validGuarantee()

	// the guarantee is not part of the memory pool yet
	suite.pool.On("Has", guarantee.ID()).Return(false)
	suite.pool.On("Add", guarantee).Return(true)

	// we are not staked
	suite.finalIdentities = suite.finalIdentities.Filter(filter.Not(filter.HasNodeID(suite.con1ID)))

	// submit the guarantee
	err := suite.core.OnGuarantee(suite.collID, guarantee)
	suite.Assert().NoError(err, "should not error on guarantee when unstaked")

	// the guarantee should be added to the mempool
	suite.pool.AssertCalled(suite.T(), "Add", guarantee)

}

func (suite *IngestionCoreSuite) TestOnGuaranteeNewFromConsensus() {

	guarantee := suite.validGuarantee()

	// the guarantee is not part of the memory pool yet
	suite.pool.On("Has", guarantee.ID()).Return(false)
	suite.pool.On("Add", guarantee).Return(true)

	// submit the guarantee as if it was sent by a consensus node
	err := suite.core.OnGuarantee(suite.con1ID, guarantee)
	suite.Assert().NoError(err, "should not error on new guarantee from consensus node")

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

	// check that the guarantee has been added to the mempool
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

func (suite *IngestionCoreSuite) TestOnGuaranteeNoGuarantor() {

	// create a guarantee signed by the collection node and referencing the
	// current head of the protocol state
	guarantee := suite.validGuarantee()
	guarantee.SignerIDs = nil

	// the guarantee is part of the memory pool
	suite.pool.On("Has", guarantee.ID()).Return(false)
	suite.pool.On("Add", guarantee).Return(false)

	// submit the guarantee as if it was sent by a consensus node
	err := suite.core.OnGuarantee(suite.collID, guarantee)
	suite.Assert().Error(err, "should error with missing guarantor")
	suite.Assert().True(engine.IsInvalidInputError(err))

	// check that the guarantee has been added to the mempool
	suite.pool.AssertNotCalled(suite.T(), "Add", guarantee)

}

func (suite *IngestionCoreSuite) TestOnGuaranteeInvalidRole() {

	// create a guarantee signed by the collection node and referencing the
	// current head of the protocol state
	guarantee := suite.validGuarantee()
	guarantee.SignerIDs = append(guarantee.SignerIDs, suite.execID)

	// the guarantee is part of the memory pool
	suite.pool.On("Has", guarantee.ID()).Return(false)
	suite.pool.On("Add", guarantee).Return(false)

	// submit the guarantee as if it was sent by a consensus node
	err := suite.core.OnGuarantee(suite.collID, guarantee)
	suite.Assert().Error(err, "should error with missing guarantor")
	suite.Assert().True(engine.IsInvalidInputError(err))

	// check that the guarantee has been added to the mempool
	suite.pool.AssertNotCalled(suite.T(), "Add", guarantee)

}

func (suite *IngestionCoreSuite) TestOnGuaranteeExpired() {

	// create an alternative block
	header := unittest.BlockHeaderFixture()
	header.Height = suite.head.Height - flow.DefaultTransactionExpiry - 1
	suite.headers.On("ByBlockID", header.ID()).Return(&header, nil)

	// create a guarantee signed by the collection node and referencing the
	// current head of the protocol state
	guarantee := suite.validGuarantee()
	guarantee.ReferenceBlockID = header.ID()

	// the guarantee is part of the memory pool
	suite.pool.On("Has", guarantee.ID()).Return(false)
	suite.pool.On("Add", guarantee).Return(false)

	// submit the guarantee as if it was sent by a consensus node
	err := suite.core.OnGuarantee(suite.collID, guarantee)
	suite.Assert().Error(err, "should error with expired collection")
	suite.Assert().True(engine.IsOutdatedInputError(err))

}

func (suite *IngestionCoreSuite) TestOnGuaranteeInvalidGuarantor() {

	// create a guarantee signed by the collection node and referencing the
	// current head of the protocol state
	guarantee := suite.validGuarantee()
	guarantee.SignerIDs = append(guarantee.SignerIDs, unittest.IdentifierFixture())

	// the guarantee is not part of the memory pool
	suite.pool.On("Has", guarantee.ID()).Return(false)
	suite.pool.On("Add", guarantee).Return(false)

	// submit the guarantee as if it was sent by a collection node
	err := suite.core.OnGuarantee(suite.collID, guarantee)
	suite.Assert().Error(err, "should error with invalid guarantor")
	suite.Assert().True(engine.IsInvalidInputError(err))

}

// test that just after an epoch boundary we still accept guarantees from collectors
// in clusters from the previous epoch (and collectors which are leaving the network
// at this epoch boundary).
func (suite *IngestionCoreSuite) TestOnGuaranteeEpochEnd() {

	// in the finalized state the collectors has 0 stake but is not ejected
	// this is what happens when we finalize the final block of the epoch during
	// which this node requested to unstake
	colID, ok := suite.finalIdentities.ByNodeID(suite.collID)
	suite.Require().True(ok)
	colID.Stake = 0

	guarantee := suite.validGuarantee()

	// the guarantee is not part of the memory pool
	suite.pool.On("Has", guarantee.ID()).Return(false)
	suite.pool.On("Add", guarantee).Return(true)

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
	guarantee.SignerIDs = []flow.Identifier{suite.collID}
	guarantee.ReferenceBlockID = suite.head.ID()
	return guarantee
}
