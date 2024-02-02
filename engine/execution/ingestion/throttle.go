package ingestion

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// CatchUpThreshold is the number of blocks that if the execution is far behind
// the finalization then we will only lazy load the next unexecuted finalized
// blocks until the execution has caught up
const CatchUpThreshold = 500

// BlockThrottle is a helper struct that throttles the unexecuted blocks to be sent
func NewThrottleEngine(
	log zerolog.Logger,
	handler BlockHandler,
	state protocol.State,
	execState state.ExecutionState,
	headers storage.Headers,
	catchupThreshold int,
) (*component.ComponentManager, error) {
	throttle, err := NewBlockThrottle(log, state, execState, headers, catchupThreshold)
	if err != nil {
		return nil, fmt.Errorf("could not create throttle: %w", err)
	}

	e := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			// TODO: config the buffer size
			// since the handler.OnBlock method could be blocking, we need to make sure
			// the channel has enough buffer space to hold the unprocessed blocks.
			// if the channel is full, then it will block the follower engine from
			// delivering new blocks until the channel is not full, which could be
			// useful because we probably don't want to process too many blocks if
			// the execution is not fast enough or even stopped.
			// TODO: wrap the channel so that we can report acurate metrics about the
			// buffer size
			processables := make(chan flow.Identifier, 10000)

			go func() {
				err := forwardProcessableToHandler(ctx, headers, handler, processables)
				if err != nil {
					ctx.Throw(err)
				}
			}()

			log.Info().Msg("initializing throttle engine")

			err = throttle.Init(processables)
			if err != nil {
				ctx.Throw(err)
			}

			log.Info().Msgf("throttle engine initialized")

			ready()
		}).
		Build()
	return e, nil
}

func forwardProcessableToHandler(
	ctx context.Context,
	headers storage.Headers,
	handler BlockHandler,
	processables <-chan flow.Identifier,
) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case blockID := <-processables:
			block, err := headers.ByBlockID(blockID)
			if err != nil {
				return fmt.Errorf("could not get block: %w", err)
			}

			err = handler.OnBlock(block)
			if err != nil {
				return fmt.Errorf("could not process block: %w", err)
			}
		}
	}
}

// Throttle is a helper struct that helps throttle the unexecuted blocks to be sent
// to the block queue for execution.
// It is useful for case when execution is falling far behind the finalization, in which case
// we want to throttle the blocks to be sent to the block queue for fetching data to execute
// them. Without throttle, the block queue will be flooded with blocks, and the network
// will be flooded with requests fetching collections, and the EN might quickly run out of memory.
type BlockThrottle struct {
	// config
	threshold int // catch up threshold

	// state
	mu        sync.Mutex
	executed  uint64
	finalized uint64
	inited    bool

	// notifier
	processables chan<- flow.Identifier

	// dependencies
	log     zerolog.Logger
	state   protocol.State
	headers storage.Headers
}

func NewBlockThrottle(
	log zerolog.Logger,
	state protocol.State,
	execState state.ExecutionState,
	headers storage.Headers,
	catchupThreshold int,
) (*BlockThrottle, error) {
	finalizedHead, err := state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized head: %w", err)
	}

	finalized := finalizedHead.Height
	// TODO: implement GetHighestFinalizedExecuted for execution state when storehouse
	// is not used
	executed := execState.GetHighestFinalizedExecuted()

	if executed > finalized {
		return nil, fmt.Errorf("executed finalized %v is greater than finalized %v", executed, finalized)
	}

	return &BlockThrottle{
		threshold: catchupThreshold,
		executed:  executed,
		finalized: finalized,

		log:     log.With().Str("component", "throttle").Logger(),
		state:   state,
		headers: headers,
	}, nil
}

func (c *BlockThrottle) Init(processables chan<- flow.Identifier) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inited {
		return fmt.Errorf("throttle already inited")
	}

	c.inited = true

	var unexecuted []flow.Identifier
	var err error
	if caughtUp(c.executed, c.finalized, c.threshold) {
		unexecuted, err = findAllUnexecutedBlocks(c.state, c.headers, c.executed, c.finalized)
		if err != nil {
			return err
		}
	} else {
		unexecuted, err = findFinalized(c.state, c.headers, c.executed, c.executed+500)
		if err != nil {
			return err
		}
	}

	for _, id := range unexecuted {
		c.processables <- id
	}

	c.log.Info().Msgf("throttle initialized with %d unexecuted blocks", len(unexecuted))

	return nil
}

func (c *BlockThrottle) OnBlockExecuted(_ flow.Identifier, executed uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.inited {
		return fmt.Errorf("throttle not inited")
	}

	// we have already caught up, ignore
	if c.caughtUp() {
		return nil
	}

	// the execution is still far behind from finalization
	c.executed = executed
	if !c.caughtUp() {
		return nil
	}

	c.log.Info().Uint64("executed", executed).Uint64("finalized", c.finalized).
		Msgf("execution has caught up, processing remaining unexecuted blocks")

	// if the execution have just caught up close enough to the latest finalized blocks,
	// then process all unexecuted blocks, including finalized unexecuted and pending unexecuted
	unexecuted, err := findAllUnexecutedBlocks(c.state, c.headers, c.executed, c.finalized)
	if err != nil {
		return fmt.Errorf("could not find unexecuted blocks for processing: %w", err)
	}

	c.log.Info().Int("unexecuted", len(unexecuted)).Msgf("forwarding unexecuted blocks")

	for _, id := range unexecuted {
		c.processables <- id
	}

	c.log.Info().Msgf("all unexecuted blocks have been processed")

	return nil
}

func (c *BlockThrottle) OnBlock(blockID flow.Identifier) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.inited {
		return fmt.Errorf("throttle not inited")
	}

	// ignore the block if has not caught up.
	if !c.caughtUp() {
		return nil
	}

	// if has caught up, then process the block
	c.log.Info().Str("blockID", blockID.String()).Msgf("forwarding block for processing")
	c.processables <- blockID
	c.log.Info().Str("blockID", blockID.String()).Msgf("block has been processed")

	return nil
}

func (c *BlockThrottle) OnBlockFinalized(lastFinalized *flow.Header) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.inited {
		return
	}

	if c.caughtUp() {
		return
	}

	if lastFinalized.Height <= c.finalized {
		return
	}

	c.finalized = lastFinalized.Height
}

func (c *BlockThrottle) caughtUp() bool {
	return caughtUp(c.executed, c.finalized, c.threshold)
}

func caughtUp(executed, finalized uint64, threshold int) bool {
	return finalized <= executed+uint64(threshold)
}

func findFinalized(state protocol.State, headers storage.Headers, lastExecuted, finalizedHeight uint64) ([]flow.Identifier, error) {
	// get finalized height
	finalized := state.AtHeight(finalizedHeight)
	final, err := finalized.Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized block: %w", err)
	}

	// dynamically bootstrapped execution node will have highest finalized executed as sealed root,
	// which is lower than finalized root. so we will reload blocks from
	// [sealedRoot.Height + 1, finalizedRoot.Height] and execute them on startup.
	unexecutedFinalized := make([]flow.Identifier, 0)

	// starting from the first unexecuted block, go through each unexecuted and finalized block
	// reload its block to execution queues
	// loading finalized blocks
	for height := lastExecuted + 1; height <= final.Height; height++ {
		finalizedID, err := headers.BlockIDByHeight(height)
		if err != nil {
			return nil, fmt.Errorf("could not get header at height: %v, %w", height, err)
		}

		unexecutedFinalized = append(unexecutedFinalized, finalizedID)
	}
	return unexecutedFinalized, nil
}

func findAllUnexecutedBlocks(state protocol.State, headers storage.Headers, lastExecuted, finalizedHeight uint64) ([]flow.Identifier, error) {
	unexecutedFinalized, err := findFinalized(state, headers, lastExecuted, finalizedHeight)
	if err != nil {
		return nil, fmt.Errorf("could not find finalized unexecuted blocks: %w", err)
	}

	// loaded all pending blocks
	pendings, err := state.AtHeight(finalizedHeight).Descendants()
	if err != nil {
		return nil, fmt.Errorf("could not get descendants of finalized block: %w", err)
	}

	unexecuted := append(unexecutedFinalized, pendings...)
	return unexecuted, nil
}
