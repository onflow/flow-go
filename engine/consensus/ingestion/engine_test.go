// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	mockmempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/state/protocol"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestIngestion(t *testing.T) {
	suite.Run(t, new(IngestionSuite))
}

type IngestionSuite struct {
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
	con     *mocknetwork.Conduit

	ingest *Engine
}

func (is *IngestionSuite) SetupTest() {

	head := unittest.BlockHeaderFixture()
	head.Height = 2 * flow.DefaultTransactionExpiry

	con1 := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	con2 := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	con3 := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	coll := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exec := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	is.con1ID = con1.NodeID
	is.con2ID = con2.NodeID
	is.con3ID = con3.NodeID
	is.collID = coll.NodeID
	is.execID = exec.NodeID

	clusters := flow.ClusterList{flow.IdentityList{coll}}

	identities := flow.IdentityList{con1, con2, con3, coll, exec}
	is.finalIdentities = identities.Copy()
	is.refIdentities = identities.Copy()

	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()
	state := &mockprotocol.State{}
	final := &mockprotocol.Snapshot{}
	ref := &mockprotocol.Snapshot{}
	is.query = &mockprotocol.EpochQuery{}
	is.epoch = &mockprotocol.Epoch{}
	headers := &mockstorage.Headers{}
	me := &mockmodule.Local{}
	pool := &mockmempool.Guarantees{}
	con := &mocknetwork.Conduit{}

	// this state basically works like a normal protocol state
	// returning everything correctly, using the created header
	// as head of the protocol state
	state.On("Final").Return(final)
	final.On("Head").Return(&head, nil)
	final.On("Identity", mock.Anything).Return(
		func(nodeID flow.Identifier) *flow.Identity {
			identity, _ := is.finalIdentities.ByNodeID(nodeID)
			return identity
		},
		func(nodeID flow.Identifier) error {
			_, ok := is.finalIdentities.ByNodeID(nodeID)
			if !ok {
				return protocol.IdentityNotFoundError{NodeID: nodeID}
			}
			return nil
		},
	)
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return is.finalIdentities.Filter(selector)
		},
		nil,
	)
	ref.On("Epochs").Return(is.query)
	is.query.On("Current").Return(is.epoch)
	is.epoch.On("Clustering").Return(clusters, nil)

	state.On("AtBlockID", mock.Anything).Return(ref)
	ref.On("Identity", mock.Anything).Return(
		func(nodeID flow.Identifier) *flow.Identity {
			identity, _ := is.refIdentities.ByNodeID(nodeID)
			return identity
		},
		func(nodeID flow.Identifier) error {
			_, ok := is.refIdentities.ByNodeID(nodeID)
			if !ok {
				return protocol.IdentityNotFoundError{NodeID: nodeID}
			}
			return nil
		},
	)

	// we use the first consensus node as our local identity
	me.On("NodeID").Return(is.con1ID)

	// we need to return the head as it's also used as reference block
	headers.On("ByBlockID", head.ID()).Return(&head, nil)

	// only used for metrics, nobody cares
	pool.On("Size").Return(uint(0))

	ingest := &Engine{
		tracer:  tracer,
		metrics: metrics,
		spans:   metrics,
		mempool: metrics,
		state:   state,
		headers: headers,
		me:      me,
		pool:    pool,
		con:     con,
	}

	is.head = &head
	is.final = final
	is.ref = ref
	is.headers = headers
	is.pool = pool
	is.con = con
	is.ingest = ingest
}

func (is *IngestionSuite) TestOnGuaranteeNewFromCollection() {

	// create a guarantee signed by the collection node and referencing the
	// current head of the protocol state
	guarantee := unittest.CollectionGuaranteeFixture()
	guarantee.SignerIDs = []flow.Identifier{is.collID}
	guarantee.ReferenceBlockID = is.head.ID()

	// the guarantee is not part of the memory pool yet
	is.pool.On("Has", guarantee.ID()).Return(false)
	is.pool.On("Add", guarantee).Return(true)

	is.expectGuaranteePublished(guarantee)

	// submit the guarantee as if it was sent by a collection node
	err := is.ingest.onGuarantee(is.collID, guarantee)
	is.Assert().NoError(err, "should not error on new guarantee from collection node")

	// check that the guarantee has been added to the mempool
	is.pool.AssertCalled(is.T(), "Add", guarantee)

	// check that the submit call was called
	is.con.AssertExpectations(is.T())
}

func (is *IngestionSuite) TestOnGuaranteeNewFromConsensus() {

	// create a guarantee signed by the collection node and referencing the
	// current head of the protocol state
	guarantee := unittest.CollectionGuaranteeFixture()
	guarantee.SignerIDs = []flow.Identifier{is.collID}
	guarantee.ReferenceBlockID = is.head.ID()

	// the guarantee is not part of the memory pool yet
	is.pool.On("Has", guarantee.ID()).Return(false)
	is.pool.On("Add", guarantee).Return(true)

	// submit the guarantee as if it was sent by a consensus node
	err := is.ingest.onGuarantee(is.con1ID, guarantee)
	is.Assert().NoError(err, "should not error on new guarantee from consensus node")

	// check that the guarantee has been added to the mempool
	is.pool.AssertCalled(is.T(), "Add", guarantee)

	// check that the submit call was not called
	is.con.AssertExpectations(is.T())
}

func (is *IngestionSuite) TestOnGuaranteeOld() {

	// create a guarantee signed by the collection node and referencing the
	// current head of the protocol state
	guarantee := unittest.CollectionGuaranteeFixture()
	guarantee.SignerIDs = []flow.Identifier{is.collID}
	guarantee.ReferenceBlockID = is.head.ID()

	// the guarantee is part of the memory pool
	is.pool.On("Has", guarantee.ID()).Return(true)
	is.pool.On("Add", guarantee).Return(true)

	// submit the guarantee as if it was sent by a collection node
	err := is.ingest.onGuarantee(is.collID, guarantee)
	is.Assert().NoError(err, "should not error on old guarantee")

	// check that the guarantee has been added to the mempool
	is.pool.AssertNotCalled(is.T(), "Add", guarantee)

	// check that the submit call was not called
	is.con.AssertExpectations(is.T())
}

func (is *IngestionSuite) TestOnGuaranteeNotAdded() {

	// create a guarantee signed by the collection node and referencing the
	// current head of the protocol state
	guarantee := unittest.CollectionGuaranteeFixture()
	guarantee.SignerIDs = []flow.Identifier{is.collID}
	guarantee.ReferenceBlockID = is.head.ID()

	// the guarantee is not already part of the memory pool
	is.pool.On("Has", guarantee.ID()).Return(false)
	is.pool.On("Add", guarantee).Return(false)

	// submit the guarantee as if it was sent by a collection node
	err := is.ingest.onGuarantee(is.collID, guarantee)
	is.Assert().NoError(err, "should not error when guarantee was already added")

	// check that the guarantee has been added to the mempool
	is.pool.AssertCalled(is.T(), "Add", guarantee)

	// check that the submit call was not called
	is.con.AssertExpectations(is.T())
}

func (is *IngestionSuite) TestOnGuaranteeNoGuarantor() {

	// create a guarantee signed by the collection node and referencing the
	// current head of the protocol state
	guarantee := unittest.CollectionGuaranteeFixture()
	guarantee.SignerIDs = nil
	guarantee.ReferenceBlockID = is.head.ID()

	// the guarantee is part of the memory pool
	is.pool.On("Has", guarantee.ID()).Return(false)
	is.pool.On("Add", guarantee).Return(false)

	// submit the guarantee as if it was sent by a consensus node
	err := is.ingest.onGuarantee(is.collID, guarantee)
	is.Assert().Error(err, "should error with missing guarantor")

	// check that the guarantee has been added to the mempool
	is.pool.AssertNotCalled(is.T(), "Add", guarantee)

	// check that the submit call was not called
	is.con.AssertExpectations(is.T())
}

func (is *IngestionSuite) TestOnGuaranteeInvalidRole() {

	// create a guarantee signed by the collection node and referencing the
	// current head of the protocol state
	guarantee := unittest.CollectionGuaranteeFixture()
	guarantee.SignerIDs = []flow.Identifier{is.execID}
	guarantee.ReferenceBlockID = is.head.ID()

	// the guarantee is part of the memory pool
	is.pool.On("Has", guarantee.ID()).Return(false)
	is.pool.On("Add", guarantee).Return(false)

	// submit the guarantee as if it was sent by a consensus node
	err := is.ingest.onGuarantee(is.collID, guarantee)
	is.Assert().Error(err, "should error with missing guarantor")

	// check that the guarantee has been added to the mempool
	is.pool.AssertNotCalled(is.T(), "Add", guarantee)

	// check that the submit call was not called
	is.con.AssertExpectations(is.T())
}

func (is *IngestionSuite) TestOnGuaranteeExpired() {

	// create an alternative block
	header := unittest.BlockHeaderFixture()
	header.Height = is.head.Height - flow.DefaultTransactionExpiry - 1
	is.headers.On("ByBlockID", header.ID()).Return(&header, nil)

	// create a guarantee signed by the collection node and referencing the
	// current head of the protocol state
	guarantee := unittest.CollectionGuaranteeFixture()
	guarantee.SignerIDs = []flow.Identifier{is.collID}
	guarantee.ReferenceBlockID = header.ID()

	// the guarantee is part of the memory pool
	is.pool.On("Has", guarantee.ID()).Return(false)
	is.pool.On("Add", guarantee).Return(false)

	// submit the guarantee as if it was sent by a consensus node
	err := is.ingest.onGuarantee(is.collID, guarantee)
	is.Assert().Error(err, "should error with expired collection")

	// check that the guarantee has been added to the mempool
	is.pool.AssertNotCalled(is.T(), "Add", guarantee)

	// check that the submit call was not called
	is.con.AssertExpectations(is.T())
}

func (is *IngestionSuite) TestOnGuaranteeInvalidGuarantor() {

	// create a guarantee signed by the collection node and referencing the
	// current head of the protocol state
	guarantee := unittest.CollectionGuaranteeFixture()
	guarantee.SignerIDs = []flow.Identifier{is.collID, unittest.IdentifierFixture()}
	guarantee.ReferenceBlockID = is.head.ID()

	// the guarantee is not part of the memory pool
	is.pool.On("Has", guarantee.ID()).Return(false)
	is.pool.On("Add", guarantee).Return(false)

	// submit the guarantee as if it was sent by a collection node
	err := is.ingest.onGuarantee(is.collID, guarantee)
	is.Assert().Error(err, "should error with invalid guarantor")

	// check that the guarantee has not been added to the mempool
	is.pool.AssertNotCalled(is.T(), "Add", guarantee)

	// check that the submit call was not called
	is.con.AssertExpectations(is.T())
}

// test that just after an epoch boundary we still accept guarantees from collectors
// in clusters from the previous epoch (and collectors which are leaving the network
// at this epoch boundary).
func (is *IngestionSuite) TestOnGuaranteeEpochEnd() {

	// in the finalized state the collectors has 0 stake but is not ejected
	// this is what happens when we finalize the final block of the epoch during
	// which this node requested to unstake
	colID, ok := is.finalIdentities.ByNodeID(is.collID)
	is.Require().True(ok)
	colID.Stake = 0

	guarantee := unittest.CollectionGuaranteeFixture()
	guarantee.SignerIDs = []flow.Identifier{is.collID}
	guarantee.ReferenceBlockID = is.head.ID()

	// the guarantee is not part of the memory pool
	is.pool.On("Has", guarantee.ID()).Return(false)
	is.pool.On("Add", guarantee).Return(true)

	is.expectGuaranteePublished(guarantee)

	// submit the guarantee as if it was sent by the collection node which
	// is leaving at the current epoch boundary
	err := is.ingest.onGuarantee(is.collID, guarantee)
	is.Assert().NoError(err, "should not error with collector from ending epoch")

	// check that the guarantee has been added to the mempool
	is.pool.AssertExpectations(is.T())

	// check that the Publish call was called
	is.con.AssertExpectations(is.T())
}

// expectGuaranteePublished creates an expectation on the Conduit mock that the
// guarantee should be published to the consensus nodes
func (is *IngestionSuite) expectGuaranteePublished(guarantee *flow.CollectionGuarantee) {

	// check that we call the submit with the correct consensus node IDs
	is.con.On("Publish", guarantee, mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			nodeID1 := args.Get(1).(flow.Identifier)
			nodeID2 := args.Get(2).(flow.Identifier)
			is.Assert().ElementsMatch([]flow.Identifier{nodeID1, nodeID2}, []flow.Identifier{is.con2ID, is.con3ID})
		},
	).Return(nil).Once()
}
