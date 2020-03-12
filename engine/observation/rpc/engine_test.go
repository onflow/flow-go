package rpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine/observation/rpc"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
)

type Suite struct {
	suite.Suite
}

func TestRPCEngine(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) TestPing() {
	h := rpc.Handler{}
	ping := &observation.PingRequest{}
	_, err := h.Ping(context.Background(), ping)
	suite.Require().NoError(err)
}
