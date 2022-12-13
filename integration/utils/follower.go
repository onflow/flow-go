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

	stopped chan struct{}

	// Following fields are protected by mu.
	mu       *sync.RWMutex
	height   uint64
	blockID  flowsdk.Identifier
	txToChan map[flowsdk.Identifier]txInfo
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

		stopped:  make(chan struct{}),
		interval: 100 * time.Millisecond,

		mu:       &sync.RWMutex{},
		txToChan: make(map[flowsdk.Identifier]txInfo),
	}

	for _, opt := range opts {
		opt(f)
	}

	if f.height == 0 {
		hdr, err := client.GetLatestBlockHeader(newCtx, false)
		if err != nil {
			return nil, err
		}
		f.updateFromBlockHeader(*hdr)
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

				if txi, loaded := f.loadAndDelete(tx); loaded {
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
			Int("txsInProgress", f.InProgress()).
			Msg("new block parsed")

		f.updateFromBlockHeader(block.BlockHeader)

		lastBlockTime = time.Now()
	}
}

// Follow returns a channel that will be closed when the transaction is completed.
func (f *txFollowerImpl) Follow(txID flowsdk.Identifier) <-chan struct{} {
	f.mu.Lock()
	defer f.mu.Unlock()

	select {
	case <-f.ctx.Done():
		// This channel is closed when the follower is stopped.
		return f.stopped
	default:
	}

	// Return existing follower if exists.
	if txi, ok := f.txToChan[txID]; ok {
		return txi.C
	}

	// Create new one.
	ch := make(chan struct{})
	f.txToChan[txID] = txInfo{submisionTime: time.Now(), C: ch}
	return ch
}

func (f *txFollowerImpl) loadAndDelete(txID flowsdk.Identifier) (txInfo, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	txi, ok := f.txToChan[txID]
	if ok {
		delete(f.txToChan, txID)
	}
	return txi, ok
}

func (f *txFollowerImpl) updateFromBlockHeader(block flowsdk.BlockHeader) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.height = block.Height
	f.blockID = block.ID
}

// InProgress returns the number of transactions in progress.
func (f *txFollowerImpl) InProgress() int {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return len(f.txToChan)
}

// Height returns the latest block height.
func (f *txFollowerImpl) Height() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.height
}

// BlockID returns the latest block ID.
func (f *txFollowerImpl) BlockID() flowsdk.Identifier {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.blockID
}

// Stop stops all followers, notifies existing watches, and returns.
func (f *txFollowerImpl) Stop() {
	f.cancel()
	<-f.stopped

	f.mu.Lock()
	defer f.mu.Unlock()

	for _, v := range f.txToChan {
		close(v.C)
	}
	f.txToChan = make(map[flowsdk.Identifier]txInfo)
}

type nopTxFollower struct {
	*txFollowerImpl

	closedCh chan struct{}
}

// NewNopTxFollower creates a new follower that tracks the current block height and ID
// but does not notify on transaction completion.
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

// Follow always returns a closed channel.
func (nop *nopTxFollower) Follow(ID flowsdk.Identifier) <-chan struct{} {
	return nop.closedCh
}
