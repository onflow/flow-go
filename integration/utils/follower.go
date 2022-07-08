package utils

import (
	"context"
	"sync"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access"
	"github.com/onflow/flow-go/module/metrics"

	"github.com/rs/zerolog"
)

type TxFollower interface {
	// Follow returns a channel that is closed when the transaction is complete.
	Follow(ID flowsdk.Identifier) <-chan struct{}

	// Height returns the last acted upon block height.
	Height() uint64
	// BlockID returns the last acted upon block ID.
	BlockID() flowsdk.Identifier

	// Stop sends termination signal to the follower.
	// It waits until the follower is stopped.
	Stop()
}

type followerOption func(f *txFollowerImpl)

func WithBlockHeight(height uint64) followerOption {
	return func(f *txFollowerImpl) { f.height = height }
}

func WithLogger(logger zerolog.Logger) followerOption {
	return func(f *txFollowerImpl) { f.logger = logger }
}

func WithInteval(interval time.Duration) followerOption {
	return func(f *txFollowerImpl) { f.interval = interval }
}

func WithMetrics(m *metrics.LoaderCollector) followerOption {
	return func(f *txFollowerImpl) { f.metrics = m }
}

type txFollowerImpl struct {
	logger  zerolog.Logger
	metrics *metrics.LoaderCollector

	ctx    context.Context
	cancel context.CancelFunc
	client access.Client

	interval time.Duration

	mu       *sync.RWMutex
	height   uint64
	blockID  flowsdk.Identifier
	txToChan map[flowsdk.Identifier]txInfo

	stopped chan struct{}
}

type txInfo struct {
	submisionTime time.Time

	C chan struct{}
}

// NewTxFollower creates a new follower that tracks the current block height
// and can notify on transaction completion.
func NewTxFollower(ctx context.Context, client access.Client, opts ...followerOption) (TxFollower, error) {
	newCtx, cancel := context.WithCancel(ctx)

	f := &txFollowerImpl{
		client: client,
		ctx:    newCtx,
		cancel: cancel,
		logger: zerolog.Nop(),

		mu:       &sync.RWMutex{},
		txToChan: make(map[flowsdk.Identifier]txInfo),

		stopped:  make(chan struct{}, 1),
		interval: 100 * time.Millisecond,
	}

	for _, opt := range opts {
		opt(f)
	}

	if f.height == 0 {
		hdr, err := client.GetLatestBlockHeader(newCtx, false)
		if err != nil {
			return nil, err
		}
		f.height = hdr.Height
		f.blockID = hdr.ID
	}

	go f.run()

	return f, nil
}

func (f *txFollowerImpl) run() {
	t := time.NewTicker(f.interval)
	defer t.Stop()
	defer close(f.stopped)

	var totalTxs, totalUnknownTxs uint64
Loop:
	for lastBlockTime := time.Now(); ; <-t.C {
		blockResolutionStart := time.Now()

		select {
		case <-f.ctx.Done():
			return
		default:
		}

		GetBlockByHeightTime := time.Now()
		block, err := f.client.GetBlockByHeight(f.ctx, f.height+1)
		if err != nil {
			continue
		}
		getBlockByHeightDuration := time.Since(GetBlockByHeightTime)

		var blockTxs, blockUnknownTxs uint64
		for _, guaranteed := range block.CollectionGuarantees {
			col, err := f.client.GetCollection(f.ctx, guaranteed.CollectionID)
			if err != nil {
				continue Loop
			}
			for _, tx := range col.TransactionIDs {
				blockTxs++

				f.mu.Lock()
				txi, ok := f.txToChan[tx]
				if ok {
					delete(f.txToChan, tx)
				}
				f.mu.Unlock()

				if ok {
					duration := time.Since(txi.submisionTime)
					f.logger.Trace().
						Dur("durationInMS", duration).
						Hex("txID", tx.Bytes()).
						Msg("returned account to the pool")
					close(txi.C)
					if f.metrics != nil {
						f.metrics.TransactionExecuted(duration)
					}
				} else {
					blockUnknownTxs++
				}
			}
		}

		f.mu.RLock()
		inProgress := len(f.txToChan)
		f.mu.RUnlock()

		totalTxs += blockTxs
		totalUnknownTxs += blockUnknownTxs

		f.logger.Debug().
			Uint64("blockHeight", block.Height).
			Hex("blockID", block.ID.Bytes()).
			Dur("timeSinceLastBlockInMS", time.Since(lastBlockTime)).
			Dur("timeToParseBlockInMS", time.Since(blockResolutionStart)).
			Dur("timeToGetBlockByHeightInMS", getBlockByHeightDuration).
			Int("numCollections", len(block.CollectionGuarantees)).
			Int("numSeals", len(block.Seals)).
			Uint64("txsTotal", totalTxs).
			Uint64("txsTotalUnknown", totalUnknownTxs).
			Uint64("txsInBlock", blockTxs).
			Uint64("txsInBlockUnknown", blockUnknownTxs).
			Int("txsInProgress", inProgress).
			Msg("new block parsed")

		f.mu.Lock()
		f.height = block.Height
		f.blockID = block.ID
		f.mu.Unlock()

		lastBlockTime = time.Now()
	}
}

// Follow returns a channel that will be closed when the transaction is completed.
func (f *txFollowerImpl) Follow(ID flowsdk.Identifier) <-chan struct{} {
	f.mu.Lock()
	defer f.mu.Unlock()

	select {
	case <-f.ctx.Done():
		// This channel is closed when the follower is stopped.
		return f.stopped
	default:
	}

	// Return existing follower if exists.
	if txi, ok := f.txToChan[ID]; ok {
		return txi.C
	}

	// Create new one.
	ch := make(chan struct{})
	f.txToChan[ID] = txInfo{submisionTime: time.Now(), C: ch}
	return ch
}

func (f *txFollowerImpl) Height() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.height
}

func (f *txFollowerImpl) BlockID() flowsdk.Identifier {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.blockID
}

func (f *txFollowerImpl) Stop() {
	f.cancel()
	<-f.stopped

	f.mu.Lock()
	defer f.mu.Unlock()

	for k, v := range f.txToChan {
		close(v.C)
		delete(f.txToChan, k)
	}
}

type nopTxFollower struct {
	*txFollowerImpl

	closedCh chan struct{}
}

// NewNopTxFollower creates a new follower that tracks the current block height and ID but does not notify on transaction completion.
func NewNopTxFollower(ctx context.Context, client access.Client, opts ...followerOption) (TxFollower, error) {
	f, err := NewTxFollower(ctx, client, opts...)
	if err != nil {
		return nil, err
	}
	impl, _ := f.(*txFollowerImpl)

	closedCh := make(chan struct{})
	close(closedCh)

	nop := &nopTxFollower{
		txFollowerImpl: impl,
		closedCh:       closedCh,
	}
	return nop, nil
}

// CompleteChanByID always returns a closed channel.
func (nop *nopTxFollower) Follow(ID flowsdk.Identifier) <-chan struct{} {
	return nop.closedCh
}
