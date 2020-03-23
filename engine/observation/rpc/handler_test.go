package rpc

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/protobuf/services/observation"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	mockstorage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) TestPing() {
	handler := NewHandler(zerolog.Logger{}, nil, nil, nil, nil)
	ping := &observation.PingRequest{}
	_, err := handler.Ping(context.Background(), ping)
	suite.Require().NoError(err)
}

func (suite *Suite) TestGetLatestFinalizedBlock() {
	// setup the mocks
	snapshot := new(protocol.Snapshot)
	block := unittest.BlockHeaderFixture()
	snapshot.On("Head").Return(&block, nil).Once()
	state := new(protocol.State)
	state.On("Final").Return(snapshot).Once()
	handler := NewHandler(zerolog.Logger{}, state, nil, nil, nil)

	// query the handler for the latest finalized block
	req := &observation.GetLatestBlockHeaderRequest{IsSealed: false}
	resp, err := handler.GetLatestBlockHeader(context.Background(), req)

	// make sure we got the latest block
	suite.Require().NoError(err)
	suite.Require().NotNil(resp)
	id := block.ID()
	suite.Require().Equal(id[:], resp.Block.Id)
	suite.Require().Equal(block.Height, resp.Block.Height)
	suite.Require().Equal(block.ParentID[:], resp.Block.ParentId)
	snapshot.AssertExpectations(suite.T())
	state.AssertExpectations(suite.T())
}

func (suite *Suite) TestGetLatestSealedBlock() {
	//setup the mocks
	snapshot := new(protocol.Snapshot)
	block := unittest.BlockHeaderFixture()
	seal := unittest.SealFixture()
	snapshot.On("Seal").Return(seal, nil).Once()
	headers := new(mockstorage.Headers)
	headers.On("ByBlockID", seal.BlockID).Return(&block, nil).Once()
	state := new(protocol.State)
	state.On("Final").Return(snapshot).Once()
	handler := NewHandler(zerolog.Logger{}, state, nil, nil, headers)

	// query the handler for the latest sealed block
	req := &observation.GetLatestBlockHeaderRequest{IsSealed: true}

	// make sure we got the latest sealed block
	resp, err := handler.GetLatestBlockHeader(context.Background(), req)
	suite.Require().NoError(err)
	suite.Require().NotNil(resp)
	id := block.ID()
	suite.Require().Equal(id[:], resp.Block.Id)
	suite.Require().Equal(block.Height, resp.Block.Height)
	suite.Require().Equal(block.ParentID[:], resp.Block.ParentId)
	snapshot.AssertExpectations(suite.T())
	state.AssertExpectations(suite.T())
	headers.AssertExpectations(suite.T())
}
