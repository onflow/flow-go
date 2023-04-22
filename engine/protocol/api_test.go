package protocol

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var CodesNotFoundErr = status.Errorf(codes.NotFound, "not found")
var StorageNotFoundErr = status.Errorf(codes.NotFound, "not found: %v", storage.ErrNotFound)
var InternalErr = status.Errorf(codes.Internal, "internal")
var OutputInternalErr = status.Errorf(codes.Internal, "failed to find: %v", status.Errorf(codes.Internal, "internal"))

type Suite struct {
	suite.Suite

	state            *protocol.State
	blocks           *storagemock.Blocks
	headers          *storagemock.Headers
	executionResults *storagemock.ExecutionResults
	snapshot         *protocol.Snapshot
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.snapshot = new(protocol.Snapshot)

	suite.state = new(protocol.State)
	suite.blocks = new(storagemock.Blocks)
	suite.headers = new(storagemock.Headers)
	suite.executionResults = new(storagemock.ExecutionResults)
}

func (suite *Suite) TestGetLatestFinalizedBlock_Success() {
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()

	// setup the mocks
	block := unittest.BlockFixture()
	header := block.Header

	suite.snapshot.
		On("Head").
		Return(header, nil).
		Once()

	suite.blocks.
		On("ByID", header.ID()).
		Return(&block, nil).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest finalized block
	responseBlock, err := backend.GetLatestBlock(context.Background(), false)
	suite.checkResponse(responseBlock, err)

	// make sure we got the latest block
	suite.Require().Equal(block, *responseBlock)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetLatestSealedBlock_Success() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	// setup the mocks
	block := unittest.BlockFixture()
	header := block.Header

	suite.snapshot.
		On("Head").
		Return(header, nil).
		Once()

	suite.blocks.
		On("ByID", header.ID()).
		Return(&block, nil).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest finalized block
	responseBlock, err := backend.GetLatestBlock(context.Background(), true)
	suite.checkResponse(responseBlock, err)

	// make sure we got the latest block
	suite.Require().Equal(block, *responseBlock)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetLatestBlock_StorageNotFoundFailure() {
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()

	// setup the mocks
	block := unittest.BlockFixture()
	header := block.Header

	suite.snapshot.
		On("Head").
		Return(header, storage.ErrNotFound).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest finalized block
	_, err := backend.GetLatestBlock(context.Background(), false)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, StorageNotFoundErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetLatestBlock_CodesNotFoundFailure() {
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()

	// setup the mocks
	block := unittest.BlockFixture()
	header := block.Header

	suite.snapshot.
		On("Head").
		Return(header, CodesNotFoundErr).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest finalized block
	_, err := backend.GetLatestBlock(context.Background(), false)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, CodesNotFoundErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetLatestBlock_InternalFailure() {
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()

	// setup the mocks
	block := unittest.BlockFixture()
	header := block.Header

	suite.snapshot.
		On("Head").
		Return(header, InternalErr).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest finalized block
	_, err := backend.GetLatestBlock(context.Background(), false)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, OutputInternalErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockById_Success() {
	// setup the mocks
	block := unittest.BlockFixture()

	suite.blocks.
		On("ByID", block.ID()).
		Return(&block, nil).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest sealed block
	responseBlock, err := backend.GetBlockByID(context.Background(), block.ID())
	suite.checkResponse(responseBlock, err)

	// make sure we got the latest block
	suite.Require().Equal(block.ID(), responseBlock.ID())

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockById_StorageNotFoundFailure() {
	// setup the mocks
	block := unittest.BlockFixture()

	suite.blocks.
		On("ByID", block.ID()).
		Return(&block, storage.ErrNotFound).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest block
	_, err := backend.GetBlockByID(context.Background(), block.ID())
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, StorageNotFoundErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockById_CodesNotFoundFailure() {
	// setup the mocks
	block := unittest.BlockFixture()

	suite.blocks.
		On("ByID", block.ID()).
		Return(&block, CodesNotFoundErr).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest block
	_, err := backend.GetBlockByID(context.Background(), block.ID())
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, CodesNotFoundErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockById_InternalFailure() {
	// setup the mocks
	block := unittest.BlockFixture()

	suite.blocks.
		On("ByID", block.ID()).
		Return(&block, InternalErr).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest block
	_, err := backend.GetBlockByID(context.Background(), block.ID())
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, OutputInternalErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockByHeight_Success() {
	// setup the mocks
	block := unittest.BlockFixture()
	height := block.Header.Height

	suite.blocks.
		On("ByHeight", height).
		Return(&block, nil).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest block
	responseBlock, err := backend.GetBlockByHeight(context.Background(), height)
	suite.checkResponse(responseBlock, err)

	// make sure we got the latest block
	suite.Require().Equal(block.ID(), responseBlock.ID())

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockByHeight_StorageNotFoundFailure() {
	// setup the mocks
	block := unittest.BlockFixture()
	height := block.Header.Height

	suite.blocks.
		On("ByHeight", height).
		Return(&block, StorageNotFoundErr).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest block
	_, err := backend.GetBlockByHeight(context.Background(), height)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, StorageNotFoundErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockByHeight_CodesNotFoundFailure() {
	// setup the mocks
	block := unittest.BlockFixture()
	height := block.Header.Height

	suite.blocks.
		On("ByHeight", height).
		Return(&block, CodesNotFoundErr).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest block
	_, err := backend.GetBlockByHeight(context.Background(), height)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, CodesNotFoundErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockByHeight_InternalFailure() {
	// setup the mocks
	block := unittest.BlockFixture()
	height := block.Header.Height

	suite.blocks.
		On("ByHeight", height).
		Return(&block, InternalErr).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest block
	_, err := backend.GetBlockByHeight(context.Background(), height)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, OutputInternalErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetLatestFinalizedBlockHeader_Success() {
	// setup the mocks
	blockHeader := unittest.BlockHeaderFixture()

	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Head").Return(blockHeader, nil).Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest finalized block
	responseBlockHeader, err := backend.GetLatestBlockHeader(context.Background(), false)
	suite.checkResponse(responseBlockHeader, err)

	// make sure we got the latest finalized block
	suite.Require().Equal(blockHeader.ID(), responseBlockHeader.ID())
	suite.Require().Equal(blockHeader.Height, responseBlockHeader.Height)
	suite.Require().Equal(blockHeader.ParentID, responseBlockHeader.ParentID)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetLatestSealedBlockHeader_Success() {
	// setup the mocks
	blockHeader := unittest.BlockHeaderFixture()

	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Head").Return(blockHeader, nil).Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest sealed block
	responseBlockHeader, err := backend.GetLatestBlockHeader(context.Background(), true)
	suite.checkResponse(responseBlockHeader, err)

	// make sure we got the latest sealed block
	suite.Require().Equal(blockHeader.ID(), responseBlockHeader.ID())
	suite.Require().Equal(blockHeader.Height, responseBlockHeader.Height)
	suite.Require().Equal(blockHeader.ParentID, responseBlockHeader.ParentID)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetLatestBlockHeader_StorageNotFoundFailure() {
	// setup the mocks
	blockHeader := unittest.BlockHeaderFixture()

	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Head").Return(blockHeader, storage.ErrNotFound).Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest block
	_, err := backend.GetLatestBlockHeader(context.Background(), false)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, StorageNotFoundErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetLatestBlockHeader_CodesNotFoundFailure() {
	// setup the mocks
	blockHeader := unittest.BlockHeaderFixture()

	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Head").Return(blockHeader, CodesNotFoundErr).Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest block
	_, err := backend.GetLatestBlockHeader(context.Background(), false)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, CodesNotFoundErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetLatestBlockHeader_InternalFailure() {
	// setup the mocks
	blockHeader := unittest.BlockHeaderFixture()

	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Head").Return(blockHeader, InternalErr).Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest block
	_, err := backend.GetLatestBlockHeader(context.Background(), false)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, OutputInternalErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockHeaderByID_Success() {
	// setup the mocks
	block := unittest.BlockFixture()
	header := block.Header

	suite.headers.
		On("ByBlockID", block.ID()).
		Return(header, nil).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the latest block
	responseBlockHeader, err := backend.GetBlockHeaderByID(context.Background(), block.ID())

	suite.checkResponse(responseBlockHeader, err)

	// make sure we got the latest block
	suite.Require().Equal(header.ID(), responseBlockHeader.ID())
	suite.Require().Equal(header.Height, responseBlockHeader.Height)
	suite.Require().Equal(header.ParentID, responseBlockHeader.ParentID)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockHeaderByID_StorageNotFoundFailure() {
	// setup the mocks
	block := unittest.BlockFixture()
	header := block.Header

	suite.headers.
		On("ByBlockID", block.ID()).
		Return(header, StorageNotFoundErr).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the block header
	_, err := backend.GetBlockHeaderByID(context.Background(), block.ID())
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, StorageNotFoundErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockHeaderByID_CodesNotFoundFailure() {
	// setup the mocks
	block := unittest.BlockFixture()
	header := block.Header

	suite.headers.
		On("ByBlockID", block.ID()).
		Return(header, CodesNotFoundErr).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the block header
	_, err := backend.GetBlockHeaderByID(context.Background(), block.ID())
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, CodesNotFoundErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockHeaderByID_InternalFailure() {
	// setup the mocks
	block := unittest.BlockFixture()
	header := block.Header

	suite.headers.
		On("ByBlockID", block.ID()).
		Return(header, InternalErr).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the block header
	_, err := backend.GetBlockHeaderByID(context.Background(), block.ID())
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, OutputInternalErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockHeaderByHeight_Success() {
	// setup the mocks
	blockHeader := unittest.BlockHeaderFixture()
	headerHeight := blockHeader.Height

	suite.headers.
		On("ByHeight", headerHeight).
		Return(blockHeader, nil).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the block header
	responseBlockHeader, err := backend.GetBlockHeaderByHeight(context.Background(), headerHeight)

	suite.checkResponse(responseBlockHeader, err)

	// make sure we got the block header
	suite.Require().Equal(blockHeader.Height, responseBlockHeader.Height)
	suite.Require().Equal(blockHeader.ParentID, responseBlockHeader.ParentID)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockHeaderByHeight_StorageNotFoundFailure() {
	// setup the mocks
	blockHeader := unittest.BlockHeaderFixture()
	headerHeight := blockHeader.Height

	suite.headers.
		On("ByHeight", headerHeight).
		Return(blockHeader, StorageNotFoundErr).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the block header
	_, err := backend.GetBlockHeaderByHeight(context.Background(), headerHeight)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, StorageNotFoundErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockHeaderByHeight_CodesNotFoundFailure() {
	// setup the mocks
	blockHeader := unittest.BlockHeaderFixture()
	headerHeight := blockHeader.Height

	suite.headers.
		On("ByHeight", headerHeight).
		Return(blockHeader, CodesNotFoundErr).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the block header
	_, err := backend.GetBlockHeaderByHeight(context.Background(), headerHeight)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, CodesNotFoundErr)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockHeaderByHeight_InternalFailure() {
	// setup the mocks
	blockHeader := unittest.BlockHeaderFixture()
	headerHeight := blockHeader.Height

	suite.headers.
		On("ByHeight", headerHeight).
		Return(blockHeader, InternalErr).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers, nil)

	// query the handler for the block header
	_, err := backend.GetBlockHeaderByHeight(context.Background(), headerHeight)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, OutputInternalErr)

	suite.assertAllExpectations()
}

func (suite *Suite) checkResponse(resp interface{}, err error) {
	suite.Require().NoError(err)
	suite.Require().NotNil(resp)
}

func (suite *Suite) assertAllExpectations() {
	suite.snapshot.AssertExpectations(suite.T())
	suite.state.AssertExpectations(suite.T())
	suite.blocks.AssertExpectations(suite.T())
	suite.headers.AssertExpectations(suite.T())
	suite.executionResults.AssertExpectations(suite.T())
}
