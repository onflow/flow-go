package rpc

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/protobuf/services/observation"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) TestPing() {
	handler := NewHandler(zerolog.Logger{}, nil, nil, nil, nil, nil)
	ping := &observation.PingRequest{}
	_, err := handler.Ping(context.Background(), ping)
	suite.Require().NoError(err)
}

func (suite *Suite) TestGetLatestBlock() {
	snapshot := new(protocol.Snapshot)
	block := unittest.BlockHeaderFixture()
	snapshot.On("Head").Return(&block, nil)
	state := new(protocol.State)
	state.On("Final").Return(snapshot)
	handler := NewHandler(zerolog.Logger{}, state, nil, nil, nil, nil)
	req := &observation.GetLatestBlockRequest{}
	resp, err := handler.GetLatestBlock(context.Background(), req)
	suite.Require().NoError(err)
	suite.Require().NotNil(resp)
	suite.Require().Equal(block.PayloadHash[:], resp.Block.Hash)
	suite.Require().Equal(block.Height, resp.Block.Number)
	suite.Require().Equal(block.ParentID[:], resp.Block.PreviousBlockHash)
}
