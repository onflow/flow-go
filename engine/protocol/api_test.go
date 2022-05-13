package protocol

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	state    *protocol.State
	snapshot *protocol.Snapshot
	log      zerolog.Logger

	blocks  *storagemock.Blocks
	headers *storagemock.Headers
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
	suite.Run(t, new(IdHeightSuite))
}

func (suite *Suite) SetupTest() {
	rand.Seed(time.Now().UnixNano())
	suite.log = zerolog.New(zerolog.NewConsoleWriter())
	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	header := unittest.BlockHeaderFixture()
	params := new(protocol.Params)
	params.On("Root").Return(&header, nil)
	suite.state.On("Params").Return(params).Maybe()
	suite.blocks = new(storagemock.Blocks)
	suite.headers = new(storagemock.Headers)
}

func (suite *Suite) TestGetLatestFinalizedBlockHeader() {
	// setup the mocks
	block := unittest.BlockHeaderFixture()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Head").Return(&block, nil).Once()

	backend := New(suite.state, suite.blocks, suite.headers)

	// query the handler for the latest finalized block
	header, err := backend.GetLatestBlockHeader(context.Background(), false)
	suite.checkResponse(header, err)

	// make sure we got the latest block
	suite.Require().Equal(block.ID(), header.ID())
	suite.Require().Equal(block.Height, header.Height)
	suite.Require().Equal(block.ParentID, header.ParentID)

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
}

func (suite *Suite) TestGetLatestFinalizedBlock() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()

	// setup the mocks
	expected := unittest.BlockFixture()
	header := expected.Header

	suite.snapshot.
		On("Head").
		Return(header, nil).
		Once()

	suite.blocks.
		On("ByID", header.ID()).
		Return(&expected, nil).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers)

	// query the handler for the latest finalized header
	actual, err := backend.GetLatestBlock(context.Background(), false)
	suite.checkResponse(actual, err)

	// make sure we got the latest header
	suite.Require().Equal(expected, *actual)

	suite.assertAllExpectations()
}

type IdHeightSuite struct {
	suite.Suite

	state    *protocol.State
	snapshot *protocol.Snapshot
	log      zerolog.Logger

	blocks  *storagemock.Blocks
	headers *storagemock.Headers

	chainID flow.ChainID
}

func (suite *IdHeightSuite) SetupTest() {
	rand.Seed(time.Now().UnixNano())
	suite.log = zerolog.New(zerolog.NewConsoleWriter())
	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)

	header := unittest.BlockHeaderFixture()
	params := new(protocol.Params)

	params.On("Root").Return(&header, nil)
	suite.state.On("Params").Return(params).Maybe()
	suite.blocks = new(storagemock.Blocks)
	suite.headers = new(storagemock.Headers)
}

func (suite *IdHeightSuite) TestGetBlockHeaderByID() {
	//setup the mocks
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	block := unittest.BlockHeaderFixture()

	suite.snapshot.On("Head").Return(&block, nil).Once()

	backend := New(suite.state, suite.blocks, suite.headers)

	// query the handler for the latest sealed block
	header, err := backend.GetBlockHeaderByID(context.Background(), block.ID())

	suite.checkResponse(header, err)

	// make sure we got the latest sealed block
	suite.Require().Equal(block.ID(), header.ID())
	suite.Require().Equal(block.Height, header.Height)
	suite.Require().Equal(block.ParentID, header.ParentID)

	suite.assertAllExpectations()
}

func (suite *IdHeightSuite) TestGetBlockHeaderByHeight() {
	//setup the mocks
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	block := unittest.BlockHeaderFixture()

	suite.snapshot.On("Head").Return(&block, nil).Once()

	backend := New(suite.state, suite.blocks, suite.headers)

	// query the handler for the latest sealed block
	header, err := backend.GetBlockHeaderByHeight(context.Background(), block.Height)

	suite.checkResponse(header, err)

	// make sure we got the latest sealed block
	suite.Require().Equal(block.ID(), header.ID())
	suite.Require().Equal(block.Height, header.Height)
	suite.Require().Equal(block.ParentID, header.ParentID)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockByHeight() {
	// setup the mocks
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	block := unittest.BlockHeaderFixture()
	suite.snapshot.On("Head").Return(&block, nil).Once()

	backend := New(suite.state, suite.blocks, suite.headers)

	// query the handler for the latest sealed block
	header, err := backend.GetBlockByHeight(context.Background(), block.Height)
	suite.checkResponse(header, err)

	// make sure we got the latest sealed block
	suite.Require().Equal(block.ID(), header.ID())

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetBlockById() {
	// setup the mocks
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	block := unittest.BlockHeaderFixture()
	suite.snapshot.On("Head").Return(&block, nil).Once()

	backend := New(suite.state, suite.blocks, suite.headers)

	// query the handler for the latest sealed block
	header, err := backend.GetBlockByID(context.Background(), block.ID())
	suite.checkResponse(header, err)

	// make sure we got the latest sealed block
	suite.Require().Equal(block.ID(), header.ID())

	suite.assertAllExpectations()
}

func (suite *IdHeightSuite) checkResponse(resp interface{}, err error) {
	suite.Require().NoError(err)
	suite.Require().NotNil(resp)
}

func (suite *IdHeightSuite) assertAllExpectations() {
	suite.snapshot.AssertExpectations(suite.T())
	suite.state.AssertExpectations(suite.T())
	suite.blocks.AssertExpectations(suite.T())
	suite.headers.AssertExpectations(suite.T())
}