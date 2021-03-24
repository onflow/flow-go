package test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/engine/verification/assigner"
	"github.com/onflow/flow-go/engine/verification/assigner/blockconsumer"
	"github.com/onflow/flow-go/engine/verification/fetcher"
	mockfetcher "github.com/onflow/flow-go/engine/verification/fetcher/mock"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func withBlockConsumer(t *testing.T, workerCount int, withConsumer func(*blockconsumer.BlockConsumer, *sync.WaitGroup)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		maxProcessing := int64(workerCount)

		processedHeight := bstorage.NewConsumerProgress(db, module.ConsumeProgressVerificationBlockHeight)
		processedIndex := bstorage.NewConsumerProgress(db, module.ConsumeProgressVerificationChunkIndex)
		collector := &metrics.NoopCollector{}
		tracer := &trace.NoopTracer{}
		participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
		s := testutil.CompleteStateFixture(t, collector, tracer, participants)
		me := testutil.LocalFixture(t, participants.Filter(filter.HasRole(flow.RoleVerification))[0])

		// chunk consumer and processor
		chunksQueue := bstorage.NewChunkQueue(db)
		chunkProcessor, chunksWg := mockChunkProcessor(t, flow.IdentifierList{}, true)
		chunkConsumer := fetcher.NewChunkConsumer(unittest.Logger(),
			processedIndex,
			chunksQueue,
			chunkProcessor,
			maxProcessing)

		// assigner engine
		chunkAssigner := &mock.ChunkAssigner{}
		assignerEng := assigner.New(unittest.Logger(),
			collector,
			tracer,
			me,
			s.State,
			chunkAssigner,
			chunksQueue,
			chunkConsumer)

		blockConsumer, _, err := blockconsumer.NewBlockConsumer(unittest.Logger(),
			processedHeight,
			s.Storage.Blocks,
			s.State,
			assignerEng,
			maxProcessing)
		require.NoError(t, err)

		withConsumer(blockConsumer, chunksWg)
	})
}

// mockChunkProcessor sets up a mock chunk processor that asserts the followings:
// - in a staked verification node:
// -- that a set of chunk locators are delivered to it.
// -- that each chunk locator is delivered only once.
// - in an unstaked verification node:
// -- no chunk locator is passed to it.
//
// mockChunkProcessor returns the mock chunk processor and a wait group that unblocks when all expected locators received.
func mockChunkProcessor(t testing.TB, expectedLocatorIDs flow.IdentifierList,
	staked bool) (*mockfetcher.AssignedChunkProcessor, *sync.WaitGroup) {
	processor := &mockfetcher.AssignedChunkProcessor{}

	// keeps track of which locators it has received
	receivedLocators := make(map[flow.Identifier]struct{})

	var (
		// decrements the wait group per distinct chunk locator received
		wg sync.WaitGroup
		// serializes processing locators (just for sake of test)
		mu sync.Mutex
	)

	if staked {
		// in staked mode, it expects chunk locators coming
		wg.Add(len(expectedLocatorIDs))
	}

	var notifier module.ProcessingNotifier
	processor.On("WithChunkConsumerNotifier", testifymock.Anything).Run(func(args testifymock.Arguments) {
		processingNotifier, ok := args[0].(module.ProcessingNotifier)
		require.True(t, ok)
		notifier = processingNotifier
	})

	processor.On("ProcessAssignedChunk", testifymock.Anything).Run(func(args testifymock.Arguments) {
		mu.Lock()
		defer mu.Unlock()

		// chunk processor (i.e., fetcher engine) should only receive a locator if the verification node is staked.
		require.True(t, staked, "unstaked fetcher engine received chunk locator")

		// the received entity should be an chunk locator
		locator, ok := args[0].(*chunks.Locator)
		assert.True(t, ok)

		locatorID := locator.ID()

		// verifies that it has not seen this locator
		_, duplicate := receivedLocators[locatorID]
		require.False(t, duplicate, fmt.Sprintf("chunk processor received duplicate locator: %x", locatorID))

		// ensures the received locator matches one we expect
		require.Contains(t, expectedLocatorIDs, locatorID, fmt.Sprintf("chunk processor unexpected locator: %x", locatorID))
	})

	return processor, &wg
}
