package rpc

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	entities "github.com/dapperlabs/flow-go/protobuf/sdk/entities"
	access "github.com/dapperlabs/flow-go/protobuf/services/access"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"

	realstorage "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	state    *protocol.State
	snapshot *protocol.Snapshot
	log      zerolog.Logger

	blocks       *storage.Blocks
	headers      *storage.Headers
	collections  *storage.Collections
	transactions *storage.Transactions
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.log = zerolog.Logger{}
	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.blocks = new(storage.Blocks)
	suite.headers = new(storage.Headers)
	suite.transactions = new(storage.Transactions)
	suite.collections = new(storage.Collections)
}

func (suite *Suite) TestPing() {
	handler := NewHandler(suite.log, nil, nil, nil, nil, nil, nil, nil)
	ping := &access.PingRequest{}
	pong, err := handler.Ping(context.Background(), ping)
	suite.checkResponse(pong, err)
}

func (suite *Suite) TestGetLatestFinalizedBlockHeader() {
	// setup the mocks
	block := unittest.BlockHeaderFixture()
	suite.snapshot.On("Head").Return(&block, nil).Once()
	handler := NewHandler(suite.log, suite.state, nil, nil, nil, nil, nil, nil)

	// query the handler for the latest finalized block
	req := &access.GetLatestBlockHeaderRequest{IsSealed: false}
	resp, err := handler.GetLatestBlockHeader(context.Background(), req)
	suite.checkResponse(resp, err)

	// make sure we got the latest block
	id := block.ID()
	suite.Require().Equal(id[:], resp.Block.Id)
	suite.Require().Equal(block.Height, resp.Block.Height)
	suite.Require().Equal(block.ParentID[:], resp.Block.ParentId)
	suite.assertAllExpectations()
}

func (suite *Suite) TestGetLatestSealedBlockHeader() {
	//setup the mocks
	block := unittest.BlockHeaderFixture()
	seal := unittest.SealFixture()
	suite.snapshot.On("Seal").Return(seal, nil).Once()

	suite.headers.On("ByBlockID", seal.BlockID).Return(&block, nil).Once()
	handler := NewHandler(suite.log, suite.state, nil, nil, nil, suite.headers, nil, nil)

	// query the handler for the latest sealed block
	req := &access.GetLatestBlockHeaderRequest{IsSealed: true}
	resp, err := handler.GetLatestBlockHeader(context.Background(), req)
	suite.checkResponse(resp, err)

	// make sure we got the latest sealed block
	id := block.ID()
	suite.Require().Equal(id[:], resp.Block.Id)
	suite.Require().Equal(block.Height, resp.Block.Height)
	suite.Require().Equal(block.ParentID[:], resp.Block.ParentId)
	suite.assertAllExpectations()
}

func (suite *Suite) TestGetTransaction() {
	transaction := unittest.TransactionFixture(func(t *flow.Transaction) {
		t.Nonce = 0
		t.ComputeLimit = 0
	})
	expected := transaction.TransactionBody
	suite.transactions.On("ByID", transaction.ID()).Return(&expected, nil).Once()
	suite.collections.On("LightByTransactionID", transaction.ID()).Return(nil, realstorage.ErrNotFound).Once()
	handler := NewHandler(suite.log, suite.state, nil, nil, nil, nil, suite.collections, suite.transactions)
	id := transaction.ID()
	req := &access.GetTransactionRequest{
		Id: id[:],
	}

	resp, err := handler.GetTransaction(context.Background(), req)
	suite.checkResponse(resp, err)

	actual, err := convert.MessageToTransaction(resp.Transaction)
	suite.checkResponse(resp, err)
	suite.Require().Equal(expected, actual)
	suite.assertAllExpectations()
}

func (suite *Suite) TestGetCollection() {
	collection := unittest.CollectionFixture(1)

	expectedIDs := make([]flow.Identifier, len(collection.Transactions))

	for i, t := range collection.Transactions {
		t.Nonce = 0 // obs api doesn't exposes nonce and compute limit as part of a transaction
		t.ComputeLimit = 0
		expectedIDs[i] = t.ID()
	}

	light := collection.Light()
	suite.collections.On("LightByID", collection.ID()).Return(&light, nil).Once()
	for _, t := range collection.Transactions {
		suite.transactions.On("ByID", t.ID()).Return(t, nil).Once()
	}
	handler := NewHandler(suite.log, suite.state, nil, nil, nil, nil, suite.collections, suite.transactions)
	id := collection.ID()
	req := &access.GetCollectionByIDRequest{
		Id: id[:],
	}

	resp, err := handler.GetCollectionByID(context.Background(), req)
	suite.transactions.AssertExpectations(suite.T())
	suite.checkResponse(resp, err)

	actualColl := resp.Collection
	actualIDs := make([]flow.Identifier, len(actualColl.TransactionIds))
	for i, t := range actualColl.TransactionIds {
		actualIDs[i] = flow.HashToID(t)
	}

	suite.ElementsMatch(expectedIDs, actualIDs)
	suite.assertAllExpectations()
}

// TestTransactionStatusTransition tests that the status of transaction changes from Finalized to Sealed
// when the protocol state is updated
func (suite *Suite) TestTransactionStatusTransition() {

	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]
	block := unittest.BlockFixture()
	block.Height = 2
	headBlock := unittest.BlockFixture()
	headBlock.Height = block.Height - 1 // head is behind the current block

	seal := unittest.SealFixture()
	seal.BlockID = headBlock.ID()
	suite.snapshot.On("Seal").Return(seal, nil).Twice()
	suite.headers.On("ByBlockID", seal.BlockID).Return(&headBlock.Header, nil).Twice()

	light := collection.Light()
	// transaction storage returns the corresponding transaction
	suite.transactions.On("ByID", transactionBody.ID()).Return(transactionBody, nil).Twice()
	// collection storage returns the corresponding collection
	suite.collections.On("LightByTransactionID", transactionBody.ID()).Return(&light, nil).Twice()
	// block storage returns the corresponding block
	suite.blocks.On("ByCollectionID", collection.ID()).Return(&block, nil).Twice()

	handler := NewHandler(suite.log, suite.state, nil, nil, suite.blocks, suite.headers, suite.collections, suite.transactions)
	id := transactionBody.ID()
	req := &access.GetTransactionRequest{
		Id: id[:],
	}

	resp, err := handler.GetTransaction(context.Background(), req)
	suite.checkResponse(resp, err)

	// status should be finalized since the sealed blocks is smaller in height
	suite.Assert().Equal(entities.TransactionStatus_STATUS_FINALIZED, resp.Transaction.Status)

	// now let the head block be finalized
	headBlock.Height = block.Height + 1

	resp, err = handler.GetTransaction(context.Background(), req)
	suite.checkResponse(resp, err)

	// status should be sealed since the sealed blocks is greater in height
	suite.Assert().Equal(entities.TransactionStatus_STATUS_SEALED, resp.Transaction.Status)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetLatestFinalizedBlock() {
	// setup the mocks
	block := unittest.BlockFixture()
	header := block.Header
	suite.snapshot.On("Head").Return(&header, nil).Once()
	suite.blocks.On("ByID", header.ID()).Return(&block, nil).Once()
	handler := NewHandler(suite.log, suite.state, nil, nil, suite.blocks, nil, nil, nil)

	// query the handler for the latest finalized header
	req := &access.GetLatestBlockRequest{IsSealed: false}
	resp, err := handler.GetLatestBlock(context.Background(), req)
	suite.checkResponse(resp, err)

	// make sure we got the latest header
	expected, err := convert.BlockToMessage(&block)
	suite.Require().NoError(err)
	suite.Require().Equal(expected, resp.Block)
	suite.assertAllExpectations()
}

func (suite *Suite) assertAllExpectations() {
	suite.snapshot.AssertExpectations(suite.T())
	suite.state.AssertExpectations(suite.T())
	suite.blocks.AssertExpectations(suite.T())
	suite.headers.AssertExpectations(suite.T())
	suite.collections.AssertExpectations(suite.T())
	suite.transactions.AssertExpectations(suite.T())
}

func (suite *Suite) checkResponse(resp interface{}, err error) {
	suite.Require().NoError(err)
	suite.Require().NotNil(resp)
}
