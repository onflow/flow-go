package synchronization

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/state/protocol"
)

// FinalizedHeaderCache represents the cached value of the latest finalized header.
// It is used in Engine to access latest valid data.
type FinalizedHeaderCache struct {
	mu sync.RWMutex

	log                       zerolog.Logger
	state                     protocol.State
	lastFinalizedHeader       *flow.Header
	finalizationEventNotifier engine.Notifier // notifier for finalization events

	lm      *lifecycle.LifecycleManager
	stopped chan struct{}
}

// NewFinalizedHeaderCache creates a new finalized header cache.
func NewFinalizedHeaderCache(log zerolog.Logger, state protocol.State, finalizationDistributor *pubsub.FinalizationDistributor) (*FinalizedHeaderCache, error) {
	cache := &FinalizedHeaderCache{
		state:                     state,
		lm:                        lifecycle.NewLifecycleManager(),
		log:                       log.With().Str("component", "finalized_snapshot_cache").Logger(),
		finalizationEventNotifier: engine.NewNotifier(),
		stopped:                   make(chan struct{}),
	}

	snapshot, err := cache.getHeader()
	if err != nil {
		return nil, fmt.Errorf("could not apply last finalized state")
	}

	cache.lastFinalizedHeader = snapshot

	finalizationDistributor.AddOnBlockFinalizedConsumer(cache.onFinalizedBlock)

	return cache, nil
}

// Get returns the last locally cached finalized header.
func (f *FinalizedHeaderCache) Get() *flow.Header {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.lastFinalizedHeader
}

func (f *FinalizedHeaderCache) getHeader() (*flow.Header, error) {
	finalSnapshot := f.state.Final()
	head, err := finalSnapshot.Head()
	if err != nil {
		return nil, fmt.Errorf("could not get last finalized header: %w", err)
	}

	return head, nil
}

// updateHeader updates latest locally cached finalized header.
func (f *FinalizedHeaderCache) updateHeader() error {
	f.log.Debug().Msg("updating header")

	head, err := f.getHeader()
	if err != nil {
		f.log.Err(err).Msg("failed to get header")
		return err
	}

	f.log.Debug().
		Str("block_id", head.ID().String()).
		Uint64("height", head.Height).
		Msg("got new header")

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.lastFinalizedHeader.Height < head.Height {
		f.lastFinalizedHeader = head
	}

	return nil
}

func (f *FinalizedHeaderCache) Ready() <-chan struct{} {
	f.lm.OnStart(func() {
		go f.finalizationProcessingLoop()
	})
	return f.lm.Started()
}

func (f *FinalizedHeaderCache) Done() <-chan struct{} {
	f.lm.OnStop(func() {
		<-f.stopped
	})
	return f.lm.Stopped()
}

// onFinalizedBlock implements the `OnFinalizedBlock` callback from the `hotstuff.FinalizationConsumer`
//  (1) Updates local state of last finalized snapshot.
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (f *FinalizedHeaderCache) onFinalizedBlock(block *model.Block) {
	f.log.Debug().Str("block_id", block.BlockID.String()).Msg("received new block finalization callback")
	// notify that there is new finalized block
	f.finalizationEventNotifier.Notify()
}

// finalizationProcessingLoop is a separate goroutine that performs processing of finalization events
func (f *FinalizedHeaderCache) finalizationProcessingLoop() {
	defer close(f.stopped)

	f.log.Debug().Msg("starting finalization processing loop")
	notifier := f.finalizationEventNotifier.Channel()
	for {
		select {
		case <-f.lm.ShutdownSignal():
			return
		case <-notifier:
			err := f.updateHeader()
			if err != nil {
				f.log.Fatal().Err(err).Msg("could not process latest finalized block")
			}
		}
	}
}
