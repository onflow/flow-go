package ingestion

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	enginePkg "github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/ingestion/mocks"
	"github.com/onflow/flow-go/engine/execution/ingestion/stop"
	stateMock "github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	storageerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	unittestMocks "github.com/onflow/flow-go/utils/unittest/mocks"
)

func TestIngestionCoreExecuteBlock(t *testing.T) {
	// Given R <- 1 <- 2 (Col0) <- 3 <- 4 (Col1)
	blocks, cols := makeBlocksAndCollections(t)
	// create core
	core, throttle, state, collectionDB, blocksDB, headers, fetcher, consumer :=
		createCore(t, blocks)

	// start the core
	ctx, cancel := context.WithCancel(context.Background())
	irrecoverableCtx, _ := irrecoverable.WithSignaler(ctx)
	core.Start(irrecoverableCtx)
	<-core.Ready()
	defer func() {
		cancel()
		<-core.Done()
		log.Info().Msgf("done")
	}()

	waitTime := 10 * time.Millisecond
	// Receive Block1
	// verify Block1 is executed
	wg := &sync.WaitGroup{}
	receiveBlock(t, throttle, state, headers, blocksDB, consumer, blocks[1], wg)
	verifyBlockExecuted(t, consumer, wg, blocks[1])

	// Receive Block 2 and 3, no block is executed
	receiveBlock(t, throttle, state, headers, blocksDB, consumer, blocks[2], wg)
	time.Sleep(waitTime)
	verifyBlockNotExecuted(t, consumer, blocks[2])

	receiveBlock(t, throttle, state, headers, blocksDB, consumer, blocks[3], wg)
	time.Sleep(waitTime)
	verifyBlockNotExecuted(t, consumer, blocks[3])

	// Receive Col0
	// Verify block 2 and 3 are executed
	receiveCollection(t, fetcher, core, cols[0])
	time.Sleep(waitTime)
	verifyBlockExecuted(t, consumer, wg, blocks[2], blocks[3])

	// Store Col1
	// Receive block 4
	// Verify block 4 is executed because Col1 can be found in local
	storeCollection(t, collectionDB, cols[1])
	receiveBlock(t, throttle, state, headers, blocksDB, consumer, blocks[4], wg)
	verifyBlockExecuted(t, consumer, wg, blocks[4])
}

func createCore(t *testing.T, blocks []*flow.Block) (
	*Core, Throttle, *unittestMocks.ProtocolState, *mocks.MockCollectionStore,
	*storage.Blocks, *headerStore, *mockFetcher, *mockConsumer) {
	headers := newHeadersWithBlocks(toHeaders(blocks))
	blocksDB := storage.NewBlocks(t)
	collections := mocks.NewMockCollectionStore()
	state := unittestMocks.NewProtocolState()
	require.NoError(t, state.Bootstrap(blocks[0], nil, nil))
	execState := stateMock.NewExecutionState(t)
	execState.On("GetHighestFinalizedExecuted").Return(blocks[0].Header.Height, nil)

	// root block is executed
	consumer := newMockConsumer(blocks[0].Header.ID())

	execState.On("StateCommitmentByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) (flow.StateCommitment, error) {
			executed := consumer.MockIsBlockExecuted(blockID)
			if executed {
				return unittest.StateCommitmentFixture(), nil
			}
			return flow.DummyStateCommitment, storageerr.ErrNotFound
		})

	execState.On("IsBlockExecuted", mock.Anything, mock.Anything).Return(func(height uint64, blockID flow.Identifier) (bool, error) {
		return consumer.MockIsBlockExecuted(blockID), nil
	})
	execState.On("SaveExecutionResults", mock.Anything, mock.Anything).Return(nil)

	throttle, err := NewBlockThrottle(unittest.Logger(), state, execState, headers)
	require.NoError(t, err)

	unit := enginePkg.NewUnit()
	stopControl := stop.NewStopControl(
		unit,
		time.Second,
		zerolog.Nop(),
		execState,
		headers,
		nil,
		nil,
		&flow.Header{Height: 1},
		false,
		false,
	)
	collectionFetcher := newMockFetcher()
	executor := &mockExecutor{t: t, consumer: consumer}
	metrics := metrics.NewNoopCollector()
	core, err := NewCore(unittest.Logger(), throttle, execState, stopControl, blocksDB,
		collections, executor, collectionFetcher, consumer, metrics)
	require.NoError(t, err)
	return core, throttle, state, collections, blocksDB, headers, collectionFetcher, consumer
}

func makeBlocksAndCollections(t *testing.T) ([]*flow.Block, []*flow.Collection) {
	cs := unittest.CollectionListFixture(2)
	col0, col1 := cs[0], cs[1]

	genesis := unittest.GenesisFixture()
	blocks := unittest.ChainFixtureFrom(4, genesis.Header)

	bs := append([]*flow.Block{genesis}, blocks...)
	unittest.AddCollectionsToBlock(bs[2], []*flow.Collection{col0})
	unittest.AddCollectionsToBlock(bs[4], []*flow.Collection{col1})
	unittest.RechainBlocks(bs)

	return bs, cs
}

func receiveBlock(t *testing.T, throttle Throttle, state *unittestMocks.ProtocolState, headers *headerStore, blocksDB *storage.Blocks, consumer *mockConsumer, block *flow.Block, wg *sync.WaitGroup) {
	require.NoError(t, state.Extend(block))
	blocksDB.On("ByID", block.ID()).Return(block, nil)
	require.NoError(t, throttle.OnBlock(block.ID(), block.Header.Height))
	consumer.WaitForExecuted(block.ID(), wg)
}

func verifyBlockExecuted(t *testing.T, consumer *mockConsumer, wg *sync.WaitGroup, blocks ...*flow.Block) {
	// Wait until blocks are executed
	unittest.AssertReturnsBefore(t, func() { wg.Wait() }, time.Millisecond*20)
	for _, block := range blocks {
		require.True(t, consumer.MockIsBlockExecuted(block.ID()))
	}
}

func verifyBlockNotExecuted(t *testing.T, consumer *mockConsumer, blocks ...*flow.Block) {
	for _, block := range blocks {
		require.False(t, consumer.MockIsBlockExecuted(block.ID()))
	}
}

func storeCollection(t *testing.T, collectionDB *mocks.MockCollectionStore, collection *flow.Collection) {
	log.Info().Msgf("collectionDB: store collection %v", collection.ID())
	require.NoError(t, collectionDB.Store(collection))
}

func receiveCollection(t *testing.T, fetcher *mockFetcher, core *Core, collection *flow.Collection) {
	require.True(t, fetcher.IsFetched(collection.ID()))
	core.OnCollection(collection)
}

type mockExecutor struct {
	t        *testing.T
	consumer *mockConsumer
}

func (m *mockExecutor) ExecuteBlock(ctx context.Context, block *entity.ExecutableBlock) (*execution.ComputationResult, error) {
	result := testutil.ComputationResultFixture(m.t)
	result.ExecutableBlock = block
	result.ExecutionResult.BlockID = block.ID()
	log.Info().Msgf("mockExecutor: block %v executed", block.Block.Header.Height)
	return result, nil
}

type mockConsumer struct {
	sync.Mutex
	executed map[flow.Identifier]struct{}
	wgs      map[flow.Identifier]*sync.WaitGroup
}

func newMockConsumer(executed flow.Identifier) *mockConsumer {
	return &mockConsumer{
		executed: map[flow.Identifier]struct{}{
			executed: {},
		},
		wgs: make(map[flow.Identifier]*sync.WaitGroup),
	}
}

func (m *mockConsumer) BeforeComputationResultSaved(ctx context.Context, result *execution.ComputationResult) {
}

func (m *mockConsumer) OnComputationResultSaved(ctx context.Context, result *execution.ComputationResult) string {
	m.Lock()
	defer m.Unlock()
	blockID := result.BlockExecutionResult.ExecutableBlock.ID()
	if _, ok := m.executed[blockID]; ok {
		return fmt.Sprintf("block %v is already executed", blockID)
	}
	m.executed[blockID] = struct{}{}
	log.Info().Uint64("height", result.BlockExecutionResult.ExecutableBlock.Block.Header.Height).Msg("mockConsumer: block result saved")
	m.wgs[blockID].Done()
	return ""
}

func (m *mockConsumer) WaitForExecuted(blockID flow.Identifier, wg *sync.WaitGroup) {
	m.Lock()
	defer m.Unlock()
	wg.Add(1)
	m.wgs[blockID] = wg
}

func (m *mockConsumer) MockIsBlockExecuted(id flow.Identifier) bool {
	m.Lock()
	defer m.Unlock()
	_, ok := m.executed[id]
	return ok
}

type mockFetcher struct {
	sync.Mutex
	fetching map[flow.Identifier]struct{}
}

func newMockFetcher() *mockFetcher {
	return &mockFetcher{
		fetching: make(map[flow.Identifier]struct{}),
	}
}

func (f *mockFetcher) FetchCollection(blockID flow.Identifier, height uint64, guarantee *flow.CollectionGuarantee) error {
	f.Lock()
	defer f.Unlock()

	if _, ok := f.fetching[guarantee.ID()]; ok {
		return fmt.Errorf("collection %v is already fetching", guarantee.ID())
	}

	f.fetching[guarantee.ID()] = struct{}{}
	log.Info().Msgf("mockFetcher: fetching collection %v for block %v", guarantee.ID(), height)
	return nil
}

func (f *mockFetcher) Force() {
	f.Lock()
	defer f.Unlock()
}

func (f *mockFetcher) IsFetched(colID flow.Identifier) bool {
	f.Lock()
	defer f.Unlock()
	_, ok := f.fetching[colID]
	return ok
}
