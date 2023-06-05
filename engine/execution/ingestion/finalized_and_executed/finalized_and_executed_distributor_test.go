package finalized_and_executed_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	fae "github.com/onflow/flow-go/engine/execution/ingestion/finalized_and_executed"
	exeMock "github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	storageMock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFinalizedAndExecutedDistributor_AddConsumer(t *testing.T) {
	t.Run("start and stop", func(t *testing.T) {
		dist := fae.NewDistributor(
			unittest.Logger(),
			&flow.Header{},
			nil,
			nil,
			fae.DefaultDistributorConfig,
		)
		ctx, cancel := context.WithCancel(context.Background())
		ictx := irrecoverable.NewMockSignalerContext(t, ctx)

		dist.Start(ictx)

		unittest.AssertClosesBefore(t, dist.Ready(), 1*time.Second)

		cancel()

		unittest.AssertClosesBefore(t, dist.Done(), 1*time.Second)
	})

	t.Run("handle finalized and executed", func(t *testing.T) {
		headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		headerB1 := unittest.BlockHeaderWithParentFixture(headerA)  // 21
		headerB2 := unittest.BlockHeaderWithParentFixture(headerA)  // 21
		headerC1 := unittest.BlockHeaderWithParentFixture(headerB1) // 22
		headerC2 := unittest.BlockHeaderWithParentFixture(headerB2) // 22
		headerC3 := unittest.BlockHeaderWithParentFixture(headerB2) // 22

		// make sure the views are different
		// we are making the assumption that when checking for finalization the block has
		// been executed. For executed block, it must have a QC, which means its view is
		// unique
		headerB2.View = headerB1.View + 1
		headerC2.View = headerC1.View + 1
		headerC3.View = headerC2.View + 1

		execState := exeMock.NewExecutionState(t)
		isExecutedCall := func(h *flow.Header, isExecuted bool) *mock.Call {
			var returnErr error
			if !isExecuted {
				returnErr = storage.ErrNotFound
			}
			return execState.
				On("StateCommitmentByBlockID", mock.Anything, h.ID()).
				Return(flow.StateCommitment{}, returnErr)
		}
		headers := storageMock.NewHeaders(t)
		isFinalizedCall := func(height uint64, returnFinalizedHeader *flow.Header) *mock.Call {
			var returnErr error
			if returnFinalizedHeader == nil {
				returnErr = storage.ErrNotFound
			}
			return headers.
				On("ByHeight", height).
				Return(returnFinalizedHeader, returnErr)
		}

		dist := fae.NewDistributor(
			unittest.Logger(),
			headerA,
			execState,
			headers,
			fae.DefaultDistributorConfig,
		)
		ctx, cancel := context.WithCancel(context.Background())
		ictx := irrecoverable.NewMockSignalerContext(t, ctx)

		dist.Start(ictx)
		unittest.AssertClosesBefore(t, dist.Ready(), 1*time.Second)

		expected := []*flow.Header{
			headerB1,
			headerC1,
		}
		wg := sync.WaitGroup{}
		wg.Add(len(expected))
		mockConsumer := mockConsumer{
			finalizedAndExecuted: func(h *flow.Header) {
				require.Equal(t, expected[0], h)
				expected = expected[1:]
				wg.Done()
			},
		}

		dist.AddConsumer(mockConsumer)

		isExecutedCall(headerB1, false).Once()
		// finalize correct branch first
		dist.BlockFinalized(headerB1)
		// no is executed call for headerC1 because the f&e block is 2 lower than this one
		dist.BlockFinalized(headerC1)

		isFinalizedCall(headerB1.Height, headerB1).Once()
		// execute wrong branch first
		dist.BlockExecuted(headerB2)
		dist.BlockExecuted(headerC2)
		dist.BlockExecuted(headerC3)

		// execute correct branch
		// no need to call is finalized, because it is already cached.
		dist.BlockExecuted(headerB1)
		dist.BlockExecuted(headerC1)

		unittest.AssertReturnsBefore(t, wg.Wait, 1*time.Second)

		cancel()
		unittest.AssertClosesBefore(t, dist.Done(), 1*time.Second)
	})

	t.Run("concurrent finalization and execution", func(t *testing.T) {
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))

		execState := exeMock.NewExecutionState(t)
		expectIsExecutedCall := func(h *flow.Header, isExecuted bool) {
			var returnErr error
			if !isExecuted {
				returnErr = storage.ErrNotFound
			}
			execState.
				On("StateCommitmentByBlockID", mock.Anything, h.ID()).
				Return(flow.StateCommitment{}, returnErr).
				Maybe()
		}
		headers := storageMock.NewHeaders(t)
		expectIsFinalizedCall := func(height uint64, returnFinalizedHeader *flow.Header) {
			var returnErr error
			if returnFinalizedHeader == nil {
				returnErr = storage.ErrNotFound
			}
			headers.
				On("ByHeight", height).
				Return(returnFinalizedHeader, returnErr).
				Maybe()
		}

		dist := fae.NewDistributor(
			unittest.Logger(),
			header,
			execState,
			headers,
			fae.DefaultDistributorConfig,
		)
		ctx, cancel := context.WithCancel(context.Background())
		ictx := irrecoverable.NewMockSignalerContext(t, ctx)

		dist.Start(ictx)
		unittest.AssertClosesBefore(t, dist.Ready(), 1*time.Second)

		testLen := 1000
		expectedMU := sync.Mutex{}
		var expected []*flow.Header
		wg := sync.WaitGroup{}
		wg.Add(testLen)
		mockConsumer := mockConsumer{
			finalizedAndExecuted: func(h *flow.Header) {
				expectedMU.Lock()
				defer expectedMU.Unlock()

				require.Equal(t, expected[0], h)
				expected = expected[1:]
				wg.Done()
			},
		}
		addExpectedFAE := func(h *flow.Header) {
			expectedMU.Lock()
			defer expectedMU.Unlock()

			expected = append(expected, h)
		}

		dist.AddConsumer(mockConsumer)

		// generation
		// this generates a chain of headers that will be finalized and executed
		finalChan := make(chan *flow.Header)
		execChan := make(chan *flow.Header)
		go func() {
			parent := header
			for i := 0; i < testLen; i++ {

				newHeader := unittest.BlockHeaderWithParentFixture(parent)
				// make the views unique
				newHeader.View = uint64((i + 1) * testLen)

				expectIsExecutedCall(newHeader, false)
				expectIsFinalizedCall(newHeader.Height, nil)

				finalChan <- newHeader

				addExpectedFAE(newHeader)

				execChan <- newHeader
				// simulate some branching
				parentExec := parent
				for j := 0; j < j%10; j++ {
					// make the views unique
					newHeader.View = uint64((i+1)*testLen + j + 1)
					newExec := unittest.BlockHeaderWithParentFixture(parentExec)

					expectIsExecutedCall(newHeader, false)

					// add some more headers that will be executed, but not finalized
					execChan <- newExec

					// make branches deeper
					if j%2 == 0 {
						parentExec = newExec
					}
				}

				parent = newHeader
			}
		}()

		// finalization
		go func() {
			for h := range finalChan {
				expectIsFinalizedCall(h.Height, h)
				<-time.After(2 * time.Millisecond)
				dist.BlockFinalized(h)
			}
		}()

		// execution
		go func() {
			for h := range execChan {
				expectIsExecutedCall(h, true)
				<-time.After(1 * time.Millisecond)
				dist.BlockExecuted(h)
			}
		}()

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		cancel()
		unittest.AssertClosesBefore(t, dist.Done(), 1*time.Second)
	})

}

type mockConsumer struct {
	finalizedAndExecuted func(h *flow.Header)
}

func (m mockConsumer) FinalizedAndExecuted(h *flow.Header) {
	m.finalizedAndExecuted(h)
}
