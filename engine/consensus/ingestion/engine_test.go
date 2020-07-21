// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/model/flow"
	mockmempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	"github.com/dapperlabs/flow-go/module/metrics"
	mockmodule "github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/module/trace"
	mocknetwork "github.com/dapperlabs/flow-go/network/mock"
	mockprotocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	mockstorage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
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

	final   *mockprotocol.Snapshot
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

	clusters := flow.NewClusterList(1)
	clusters.Add(0, coll)

	identities := flow.IdentityList{con1, con2, con3, coll, exec}
	lookup := make(map[flow.Identifier]*flow.Identity)
	for _, identity := range identities {
		lookup[identity.NodeID] = identity
	}

	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()
	state := &mockprotocol.State{}
	final := &mockprotocol.Snapshot{}
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
			return lookup[nodeID]
		},
		nil,
	)
	final.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)
	final.On("Clusters").Return(clusters, nil)

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

	// check that we call the submit with the correct consensus node IDs
	is.con.On("Submit", guarantee, mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			nodeID1 := args.Get(1).(flow.Identifier)
			nodeID2 := args.Get(2).(flow.Identifier)
			is.Assert().ElementsMatch([]flow.Identifier{nodeID1, nodeID2}, []flow.Identifier{is.con2ID, is.con3ID})
		},
	).Return(nil).Once()

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

	// submit the guarantee as if it was sent by a consensus node
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

	// the guarantee is part of the memory pool
	is.pool.On("Has", guarantee.ID()).Return(false)
	is.pool.On("Add", guarantee).Return(false)

	// submit the guarantee as if it was sent by a consensus node
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

	// the guarantee is part of the memory pool
	is.pool.On("Has", guarantee.ID()).Return(false)
	is.pool.On("Add", guarantee).Return(false)

	// submit the guarantee as if it was sent by a consensus node
	err := is.ingest.onGuarantee(is.collID, guarantee)
	is.Assert().Error(err, "should error with invalid guarantor")

	// check that the guarantee has been added to the mempool
	is.pool.AssertNotCalled(is.T(), "Add", guarantee)

	// check that the submit call was not called
	is.con.AssertExpectations(is.T())
}
