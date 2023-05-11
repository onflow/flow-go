package synchronization

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRequestHandlerEngine(t *testing.T) {
	suite.Run(t, new(RequestHandlerEngineSuite))
}

type RequestHandlerEngineSuite struct {
	SyncBaseSuite
	engine *RequestHandlerEngine
}

func (s *RequestHandlerEngineSuite) SetupTest() {
	s.SyncBaseSuite.SetupTest()
	eng, err := NewRequestHandlerEngine(unittest.Logger(), s.metrics, s.net, s.me, s.state, s.blocks, s.core)
	require.NoError(s.T(), err)
	s.engine = eng
}

func (s *RequestHandlerEngineSuite) TestStartStop() {
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(s.T(), context.Background())
	s.engine.Start(ctx)
	unittest.AssertClosesBefore(s.T(), s.engine.Ready(), time.Second)
	cancel()
	unittest.AssertClosesBefore(s.T(), s.engine.Done(), time.Second)
}
