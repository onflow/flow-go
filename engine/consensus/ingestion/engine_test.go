// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	netint "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestIngestionEngine(t *testing.T) {
	suite.Run(t, new(IngestionSuite))
}

type IngestionSuite struct {
	IngestionCoreSuite

	con    *mocknetwork.Conduit
	net    *mocknetwork.Network
	cancel context.CancelFunc

	ingest *Engine
}

func (s *IngestionSuite) SetupTest() {
	s.IngestionCoreSuite.SetupTest()

	s.con = &mocknetwork.Conduit{}

	// set up network module mock
	s.net = &mocknetwork.Network{}
	s.net.On("Register", engine.ReceiveGuarantees, mock.Anything).Return(
		func(channel netint.Channel, engine netint.Engine) netint.Conduit {
			return s.con
		},
		nil,
	)

	// setup my own identity
	me := &mockmodule.Local{}
	me.On("NodeID").Return(s.conID) // we use the first consensus node as our local identity

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)

	metrics := metrics.NewNoopCollector()
	ingest, err := New(unittest.Logger(), metrics, s.net, me, s.core)
	require.NoError(s.T(), err)
	s.ingest = ingest
	s.ingest.Start(signalerCtx)
	<-s.ingest.Ready()
}

func (s *IngestionSuite) TearDownTest() {
	s.cancel()
	<-s.ingest.Done()
}

func (s *IngestionSuite) TestSubmittingMultipleEntries() {
	originID := s.collID
	count := uint64(15)

	processed := atomic.NewUint64(0)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for i := 0; i < int(count); i++ {
			guarantee := s.validGuarantee()
			s.pool.On("Has", guarantee.ID()).Return(false)
			s.pool.On("Add", guarantee).Run(func(args mock.Arguments) {
				processed.Add(1)
			}).Return(true)

			// execute the vote submission
			_ = s.ingest.Process(engine.ProvideCollections, originID, guarantee)
		}
		wg.Done()
	}()

	wg.Wait()

	require.Eventually(s.T(), func() bool {
		return processed.Load() == count
	}, time.Millisecond*200, time.Millisecond*20)

	s.pool.AssertExpectations(s.T())
}
