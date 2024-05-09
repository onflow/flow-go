package ingestion

import (
	"fmt"
	"sync"
	"testing"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	stateMock "github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// Given the following chain:
// 1 <- 2 <- 3 <- 4 <- 5 <- 6 <- 7 <- 8 <- 9 <- 10
// Block 6 is the last executed, block 7 is the last finalized,
// Then block 7, 8, 9, 10 will be loaded.
// When 7, 8, 9, 10 are executed, no more block will be loaded
func TestThrottleLoadAllBlocks(t *testing.T) {
	blocks := makeBlocks(t, 0, 10)
	headers := toHeaders(blocks)
	require.Len(t, blocks, 10+1)
	threshold, lastExecuted, lastFinalized := 3, 6, 7
	throttle := createThrottle(t, blocks, headers, lastExecuted, lastFinalized)
	var wg sync.WaitGroup
	processables, consumer := makeProcessables(t, &wg, HeaderToBlockIDHeight(headers[lastExecuted]))

	wg.Add(4) // load block 7,8,9,10
	// verify that after Init, the last processable is 10 (all headers are loaded)
	require.NoError(t, throttle.Init(processables, threshold))
	wg.Wait()

	require.Equal(t, HeaderToBlockIDHeight(headers[10]), consumer.LastProcessable())
	total := consumer.Total()

	// when 7-10 are executed, verify no more block is loaded
	require.NoError(t, throttle.OnBlockExecuted(headers[6].ID(), headers[6].Height))
	require.Equal(t, total, consumer.Total())

	require.NoError(t, throttle.Done())
}

// Given the following chain:
// 1 <- 2 <- 3 <- 4 <- 5 <- 6 <- 7 <- 8 <- 9 <- 10 <- 11
// 10 and 11 are not received.
// Block 1 is the last executed, Block 7 is the last finalized.
// If threshold is 3, then block 2, 3, 4 will be loaded
// When 2 is executed, block 5 will be loaded.
// When 10 is received, no block will be loaded.
// When 3 is executed, block 6 will be loaded.
// When 4 is executed, block 7, 8, 9, 10 will be loaded.
// When 5 is executed, no block is loaded
// When 11 is received, block 11 will be loaded.
func TestThrottleFallBehindCatchUp(t *testing.T) {
	allBlocks := makeBlocks(t, 0, 11)
	blocks := allBlocks[:11]
	require.Len(t, blocks, 10+1)
	headers := toHeaders(blocks)
	threshold, lastExecuted, lastFinalized := 3, 1, 7
	throttle := createThrottle(t, blocks, headers, lastExecuted, lastFinalized)
	var wg sync.WaitGroup
	processables, consumer := makeProcessables(t, &wg, HeaderToBlockIDHeight(headers[lastExecuted]))

	wg.Add(3) // load block 2,3,4
	// verify that after Init, the last processable is 4 (only the next 3 finalized blocks are loaded)
	require.NoError(t, throttle.Init(processables, threshold))
	wg.Wait()
	require.Equal(t, HeaderToBlockIDHeight(headers[4]), consumer.LastProcessable())

	// when 2 is executed, verify block 5 is loaded
	wg.Add(1)
	require.NoError(t, throttle.OnBlockExecuted(headers[2].ID(), headers[2].Height))
	wg.Wait()
	require.Equal(t, HeaderToBlockIDHeight(headers[5]), consumer.LastProcessable())

	// when 10 is received, no block is loaded
	require.NoError(t, throttle.OnBlock(headers[10].ID(), headers[10].Height))
	require.Equal(t, HeaderToBlockIDHeight(headers[5]), consumer.LastProcessable())

	// when 3 is executed, verify block 6 is loaded
	wg.Add(1)
	require.NoError(t, throttle.OnBlockExecuted(headers[3].ID(), headers[3].Height))
	wg.Wait()
	require.Equal(t, HeaderToBlockIDHeight(headers[6]), consumer.LastProcessable())

	// when 4 is executed, verify block 7, 8, 9, 10 is loaded
	wg.Add(4)
	require.NoError(t, throttle.OnBlockExecuted(headers[4].ID(), headers[4].Height))
	wg.Wait()
	require.Equal(t, HeaderToBlockIDHeight(headers[10]), consumer.LastProcessable())
	require.Equal(t, 10, consumer.Total())

	// when 5 to 10 is executed, no block is loaded
	for i := 4; i <= 10; i++ {
		require.NoError(t, throttle.OnBlockExecuted(headers[i].ID(), headers[i].Height))
	}
	wg.Wait()
	require.Equal(t, HeaderToBlockIDHeight(headers[10]), consumer.LastProcessable())
	require.Equal(t, 10, consumer.Total())

	// when 11 is received, verify block 11 is loaded
	wg.Add(1)
	require.NoError(t, throttle.OnBlock(allBlocks[11].ID(), allBlocks[11].Header.Height))
	wg.Wait()
	require.Equal(t, HeaderToBlockIDHeight(allBlocks[11].Header), consumer.LastProcessable())

	require.NoError(t, throttle.Done())
}

func makeBlocks(t *testing.T, start, count int) []*flow.Block {
	genesis := unittest.GenesisFixture()
	blocks := unittest.ChainFixtureFrom(count, genesis.Header)
	return append([]*flow.Block{genesis}, blocks...)
}

func toHeaders(blocks []*flow.Block) []*flow.Header {
	headers := make([]*flow.Header, len(blocks))
	for i, block := range blocks {
		headers[i] = block.Header
	}
	return headers
}

func createThrottle(t *testing.T, blocks []*flow.Block, headers []*flow.Header, lastExecuted, lastFinalized int) *BlockThrottle {
	log := unittest.Logger()
	headerStore := newHeadersWithBlocks(headers)

	state := mocks.NewProtocolState()
	require.NoError(t, state.Bootstrap(blocks[0], nil, nil))
	for i := 1; i < len(blocks); i++ {
		require.NoError(t, state.Extend(blocks[i]))
	}
	require.NoError(t, state.Finalize(blocks[lastFinalized].ID()))

	execState := stateMock.NewExecutionState(t)
	execState.On("GetHighestFinalizedExecuted").Return(headers[lastExecuted].Height, nil)

	throttle, err := NewBlockThrottle(log, state, execState, headerStore)
	require.NoError(t, err)
	return throttle
}

func makeProcessables(t *testing.T, wg *sync.WaitGroup, root BlockIDHeight) (chan<- BlockIDHeight, *processableConsumer) {
	processables := make(chan BlockIDHeight, MaxProcessableBlocks)
	consumer := &processableConsumer{
		headers: map[flow.Identifier]struct{}{
			root.ID: {},
		},
		last: root,
	}
	consumer.Consume(t, wg, processables)
	return processables, consumer
}

type processableConsumer struct {
	last    BlockIDHeight
	headers map[flow.Identifier]struct{}
}

func (c *processableConsumer) Consume(t *testing.T, wg *sync.WaitGroup, processables <-chan BlockIDHeight) {
	go func() {
		for block := range processables {
			_, ok := c.headers[block.ID]
			require.False(t, ok, "block %v is already processed", block.Height)
			c.headers[block.ID] = struct{}{}
			c.last = block
			log.Info().Msgf("consuming block %v, c.last: %v", block.Height, c.last.Height)
			wg.Done()
		}
	}()
}

func (c *processableConsumer) Total() int {
	return len(c.headers)
}

func (c *processableConsumer) LastProcessable() BlockIDHeight {
	return c.last
}

type headerStore struct {
	byID     map[flow.Identifier]*flow.Header
	byHeight map[uint64]*flow.Header
}

func newHeadersWithBlocks(headers []*flow.Header) *headerStore {
	byID := make(map[flow.Identifier]*flow.Header, len(headers))
	byHeight := make(map[uint64]*flow.Header, len(headers))
	for _, header := range headers {
		byID[header.ID()] = header
		byHeight[header.Height] = header
	}
	return &headerStore{
		byID:     byID,
		byHeight: byHeight,
	}
}

func (h *headerStore) BlockIDByHeight(height uint64) (flow.Identifier, error) {
	header, ok := h.byHeight[height]
	if !ok {
		return flow.Identifier{}, fmt.Errorf("block %d not found", height)
	}
	return header.ID(), nil
}

func (h *headerStore) ByBlockID(blockID flow.Identifier) (*flow.Header, error) {
	header, ok := h.byID[blockID]
	if !ok {
		return nil, fmt.Errorf("block %v not found", blockID)
	}
	return header, nil
}

func (h *headerStore) ByHeight(height uint64) (*flow.Header, error) {
	header, ok := h.byHeight[height]
	if !ok {
		return nil, fmt.Errorf("block %d not found", height)
	}
	return header, nil
}

func (h *headerStore) Exists(blockID flow.Identifier) (bool, error) {
	_, ok := h.byID[blockID]
	return ok, nil
}

func (h *headerStore) ByParentID(parentID flow.Identifier) ([]*flow.Header, error) {
	return nil, nil
}

func (h *headerStore) Store(header *flow.Header) error {
	return nil
}
