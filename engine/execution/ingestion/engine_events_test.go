package ingestion

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	executionUnittest "github.com/onflow/flow-go/engine/execution/state/unittest"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEventsHashesAreIncluded(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {
		blockSealed := unittest.BlockHeaderFixture()

		blockA := unittest.ExecutableBlockFixtureWithParent([][]flow.Identifier{{unittest.IdentifierFixture()}}, &blockSealed)
		blockA.StartState = unittest.StateCommitmentPointerFixture()

		blockB := unittest.ExecutableBlockFixtureWithParent([][]flow.Identifier{{unittest.IdentifierFixture()}}, blockA.Block.Header)
		blockB.StartState = blockA.StartState

		collectionA := blockA.Collections()[0].Collection()
		collectionB := blockB.Collections()[0].Collection()

		ctx.collections.EXPECT().ByID(collectionA.ID()).Return(&collectionA, nil).AnyTimes()
		ctx.collections.EXPECT().ByID(collectionB.ID()).Return(&collectionB, nil).AnyTimes()

		commits := make(map[flow.Identifier]flow.StateCommitment)
		commits[blockA.Block.Header.ParentID] = *blockA.StartState

		wg := sync.WaitGroup{}
		ctx.mockStateCommitsWithMap(commits)

		ctx.state.On("Sealed").Return(ctx.snapshot)
		ctx.snapshot.On("Head").Return(&blockSealed, nil)

		computationResultsA := executionUnittest.ComputationResultForBlockFixture(blockA)
		computationResultsB := executionUnittest.ComputationResultForBlockFixture(blockB)

		eventsListA := flow.EventsList{
			unittest.EventFixture("a", 0, 0, unittest.IdentifierFixture(), 23),
			unittest.EventFixture("b", 1, 2, unittest.IdentifierFixture(), 4),
		}

		eventsListB := flow.EventsList{
			unittest.EventFixture("a.a", 0, 4, unittest.IdentifierFixture(), 23),
			unittest.EventFixture("b.b", 0, 4, unittest.IdentifierFixture(), 23),
		}

		eventsListC := flow.EventsList{
			unittest.EventFixture("a.c", 1, 0, unittest.IdentifierFixture(), 2),
			unittest.EventFixture("b.c", 2, 1, unittest.IdentifierFixture(), 3),
		}

		computationResultsA.Events = []flow.EventsList{
			eventsListA, eventsListB,
		}

		eventsListsEmpty := flow.EventsList{}
		computationResultsB.Events = []flow.EventsList{
			eventsListC, eventsListsEmpty,
		}

		ctx.assertSuccessfulBlockComputation(commits, func(blockID flow.Identifier, commit flow.StateCommitment) {
			wg.Done()
		}, blockA, unittest.IdentifierFixture(), true, *blockA.StartState, computationResultsA)

		ctx.assertSuccessfulBlockComputation(commits, func(blockID flow.Identifier, commit flow.StateCommitment) {
			wg.Done()
		}, blockB, unittest.IdentifierFixture(), true, *blockB.StartState, computationResultsB)

		wg.Add(2) // wait for blocks A & B to be executed
		err := ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		err = ctx.engine.handleBlock(context.Background(), blockB.Block)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 5*time.Second)

		_, more := <-ctx.engine.Done() //wait for all the blocks to be processed
		require.False(t, more)

		_, ok := commits[blockA.ID()]
		require.True(t, ok)

		_, ok = commits[blockB.ID()]
		require.True(t, ok)

		receiptA, ok := ctx.broadcastedReceipts[blockA.ID()]
		require.True(t, ok)

		receiptB, ok := ctx.broadcastedReceipts[blockB.ID()]
		require.True(t, ok)

		require.Len(t, receiptA.ExecutionResult.Chunks, 2)
		require.Len(t, receiptB.ExecutionResult.Chunks, 2)

		eventListAHash, err := eventsListA.Hash()
		require.NoError(t, err)
		eventListBHash, err := eventsListB.Hash()
		require.NoError(t, err)
		eventListCHash, err := eventsListC.Hash()
		require.NoError(t, err)
		emptyListHash, err := eventsListsEmpty.Hash()
		require.NoError(t, err)

		require.Equal(t, eventListAHash, receiptA.ExecutionResult.Chunks[0].EventCollection)
		require.Equal(t, eventListBHash, receiptA.ExecutionResult.Chunks[1].EventCollection)

		require.Equal(t, eventListCHash, receiptB.ExecutionResult.Chunks[0].EventCollection)
		require.Equal(t, emptyListHash, receiptB.ExecutionResult.Chunks[1].EventCollection)
	})
}
