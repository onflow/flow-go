package synchronization

import (
	"context"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/rs/zerolog"
)

// finalizedSnapshot is a helper structure which contains latest finalized header and participants list
type finalizedSnapshot struct {
	head         *flow.Header
	participants flow.IdentityList
}

// FinalizedSnapshotCache represents a cached snapshot of the latest finalized header and participants list.
// It is used in Engine to access latest valid data.
type FinalizedSnapshotCache struct {
	mu sync.Mutex

	log                       zerolog.Logger
	state                     protocol.State
	identityFilter            flow.IdentityFilter
	lastFinalizedSnapshot     *finalizedSnapshot
	finalizationEventNotifier engine.Notifier // notifier for finalization events

	lm                *lifecycle.LifecycleManager
	shutdownCompleted chan struct{} // used to signal that shutdown of finalization processing loop was completed

	ctx    context.Context
	cancel context.CancelFunc
}

// NewFinalizedSnapshotCache creates a new finalized snapshot cache.
func NewFinalizedSnapshotCache(log zerolog.Logger, state protocol.State, participantsFilter flow.IdentityFilter) (*FinalizedSnapshotCache, error) {
	ctx, cancel := context.WithCancel(context.Background())

	cache := &FinalizedSnapshotCache{
		state:             state,
		identityFilter:    participantsFilter,
		lm:                lifecycle.NewLifecycleManager(),
		ctx:               ctx,
		cancel:            cancel,
		shutdownCompleted: make(chan struct{}),
		log:               log.With().Str("component", "finalized_snapshot_cache").Logger(),
	}

	err := cache.updateSnapshot()
	if err != nil {
		return nil, fmt.Errorf("could not apply last finalized state")
	}

	return cache, nil
}

// get returns last locally stored snapshot which contains final header
// and list of filtered identities
func (f *FinalizedSnapshotCache) get() *finalizedSnapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastFinalizedSnapshot
}

// updateSnapshot updates latest locally cached finalized snapshot
func (f *FinalizedSnapshotCache) updateSnapshot() error {
	finalSnapshot := f.state.Final()
	head, err := finalSnapshot.Head()
	if err != nil {
		return fmt.Errorf("could not get last finalized header: %w", err)
	}

	// get all participant nodes from the state
	participants, err := finalSnapshot.Identities(f.identityFilter)
	if err != nil {
		return fmt.Errorf("could get consensus participants at latest finalized block: %w", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.lastFinalizedSnapshot != nil && f.lastFinalizedSnapshot.head.Height >= head.Height {
		return nil
	}

	f.lastFinalizedSnapshot = &finalizedSnapshot{
		head:         head,
		participants: participants,
	}

	return nil
}

func (f *FinalizedSnapshotCache) Ready() <-chan struct{} {
	f.lm.OnStart(func() {
		go f.finalizationProcessingLoop(f.ctx)
	})
	return f.lm.Started()
}

func (f *FinalizedSnapshotCache) Done() <-chan struct{} {
	f.cancel()
	f.lm.OnStop(func() {
		// wait for finalization processing loop to shutdown
		<-f.shutdownCompleted
	})
	return f.lm.Stopped()
}

// OnFinalizedBlock implements the `OnFinalizedBlock` callback from the `hotstuff.FinalizationConsumer`
//  (1) Updates local state of last finalized snapshot.
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (f *FinalizedSnapshotCache) OnFinalizedBlock(flow.Identifier) {
	// notify that there is new finalized block
	f.finalizationEventNotifier.Notify()
}

// finalizationProcessingLoop is a separate goroutine that performs processing of finalization events
func (f *FinalizedSnapshotCache) finalizationProcessingLoop(ctx context.Context) {
	defer close(f.shutdownCompleted)
	notifier := f.finalizationEventNotifier.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-notifier:
			err := f.updateSnapshot()
			if err != nil {
				f.log.Fatal().Err(err).Msg("could not process latest finalized block")
			}
		}
	}
}
