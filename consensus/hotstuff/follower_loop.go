package hotstuff

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/logging"
)

// FollowerLoop implements interface module.HotStuffFollower.
// FollowerLoop buffers all incoming events to the hotstuff FollowerLogic, and feeds FollowerLogic one event at a time
// using a worker thread.
// Concurrency safe.
type FollowerLoop struct {
	*component.ComponentManager
	log             zerolog.Logger
	mempoolMetrics  module.MempoolMetrics
	certifiedBlocks chan *model.CertifiedBlock
	forks           Forks
}

var _ component.Component = (*FollowerLoop)(nil)
var _ module.HotStuffFollower = (*FollowerLoop)(nil)

// NewFollowerLoop creates an instance of HotStuffFollower
func NewFollowerLoop(log zerolog.Logger, mempoolMetrics module.MempoolMetrics, forks Forks) (*FollowerLoop, error) {
	// We can't afford to drop messages since it undermines liveness, but we also want to avoid blocking
	// the compliance layer. Generally, the follower loop should be able to process inbound blocks faster
	// than they pass through the compliance layer. Nevertheless, in the worst case we will fill the
	// channel and block the compliance layer's workers. Though, that should happen only if compliance
	// engine receives large number of blocks in short periods of time (e.g. when catching up).
	certifiedBlocks := make(chan *model.CertifiedBlock, 1000)

	fl := &FollowerLoop{
		log:             log.With().Str("hotstuff", "FollowerLoop").Logger(),
		mempoolMetrics:  mempoolMetrics,
		certifiedBlocks: certifiedBlocks,
		forks:           forks,
	}

	fl.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(fl.loop).
		Build()

	return fl, nil
}

// AddCertifiedBlock appends the given certified block to the tree of pending
// blocks and updates the latest finalized block (if finalization progressed).
// Unless the parent is below the pruning threshold (latest finalized view), we
// require that the parent has previously been added.
//
// Notes:
//   - Under normal operations, this method is non-blocking. The follower internally
//     queues incoming blocks and processes them in its own worker routine. However,
//     when the inbound queue is, we block until there is space in the queue. This
//     behavior is intentional, because we cannot drop blocks (otherwise, we would
//     cause disconnected blocks). Instead, we simply block the compliance layer to
//     avoid any pathological edge cases.
//   - Blocks whose views are below the latest finalized view are dropped.
//   - Inputs are idempotent (repetitions are no-ops).
func (fl *FollowerLoop) AddCertifiedBlock(certifiedBlock *model.CertifiedBlock) {
	received := time.Now()

	select {
	case fl.certifiedBlocks <- certifiedBlock:
	case <-fl.ComponentManager.ShutdownSignal():
		return
	}

	// the busy duration is measured as how long it takes from a block being
	// received to a block being handled by the event handler.
	busyDuration := time.Since(received)

	blocksQueued := uint(len(fl.certifiedBlocks))
	fl.mempoolMetrics.MempoolEntries(metrics.ResourceFollowerLoopCertifiedBlocksChannel, blocksQueued)
	fl.log.Debug().Hex("block_id", logging.ID(certifiedBlock.ID())).
		Uint64("view", certifiedBlock.View()).
		Uint("blocks_queued", blocksQueued).
		Dur("wait_time", busyDuration).
		Msg("wait time to queue inbound certified block")
}

// loop will synchronously process all events.
// All errors from FollowerLogic are fatal:
//   - known critical error: some prerequisites of the HotStuff follower have been broken
//   - unknown critical error: bug-related
func (fl *FollowerLoop) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	shutdownSignal := fl.ComponentManager.ShutdownSignal()
	for {
		select { // to ensure we are not skipping over a termination signal
		case <-shutdownSignal:
			return
		default:
		}

		select {
		case b := <-fl.certifiedBlocks:
			err := fl.forks.AddCertifiedBlock(b)
			if err != nil { // all errors are fatal
				err = fmt.Errorf("finalization logic failes to process certified block %v: %w", b.ID(), err)
				fl.log.Error().
					Hex("block_id", logging.ID(b.ID())).
					Uint64("view", b.View()).
					Err(err).
					Msg("irrecoverable follower loop error")
				ctx.Throw(err)
			}
		case <-shutdownSignal:
			return
		}
	}
}
