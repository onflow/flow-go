package benchmark

import (
	"context"
	"fmt"
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

func WithLogger(logger zerolog.Logger) followerOption {
	return func(f *txFollowerImpl) { f.logger = logger }
}

func WithInteval(interval time.Duration) followerOption {
	return func(f *txFollowerImpl) { f.interval = interval }
}

func WithMetrics(m *metrics.LoaderCollector) followerOption {
	return func(f *txFollowerImpl) { f.metrics = m }
}

// txFollowerImpl is a follower that tracks the current block height and can notify on transaction completion.
//
// On creation it starts a goroutine that periodically checks for new blocks.
// Since there is only a single goroutine that is updating the latest blockID and blockHeight synchronization there pretty relaxed.
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

	hdr, err := client.GetLatestBlockHeader(newCtx, true)
	if err != nil {
		return nil, err
	}
	f.updateFromBlockHeader(*hdr)

	go f.run()

	return f, nil
}

type txStats struct {
	txs        uint64
	unknownTxs uint64
	errorTxs   uint64
}

func (f *txFollowerImpl) processTransactions(results []*flowsdk.TransactionResult) txStats {
	txStats := txStats{
		txs: uint64(len(results)),
	}
	for _, tx := range results {
		if tx.Error != nil {
			txStats.errorTxs++
		}
		if txi, loaded := f.loadAndDelete(tx.TransactionID); loaded {
			duration := time.Since(txi.submisionTime)
			if f.logger.Trace().Enabled() {
				f.logger.Trace().
					Dur("durationInMS", duration).
					Hex("txID", tx.TransactionID.Bytes()).
					Msg("returned account to the pool")
			}
			close(txi.C)
			if f.metrics != nil {
				f.metrics.TransactionExecuted(duration)
			}
		} else {
			txStats.unknownTxs++
		}
	}
	return txStats
}

func (f *txFollowerImpl) run() {
	t := time.NewTicker(f.interval)
	defer t.Stop()
	defer close(f.stopped)

	lastBlockTime := time.Now()

	var totalStats txStats
	for {
		select {
		case <-f.ctx.Done():
			return
		case <-t.C:
		}

		blockResolutionStart := time.Now()
		hdr, results, err := f.getNextBlocksTransactions()
		if err != nil {
			f.logger.Trace().Err(err).Uint64("next_height", f.Height()+1).Msg("collections are not ready yet")
			continue
		}

		blockStats := f.processTransactions(results)
		totalStats.txs += blockStats.txs
		totalStats.unknownTxs += blockStats.unknownTxs
		totalStats.errorTxs += blockStats.errorTxs

		f.logger.Debug().
			Uint64("blockHeight", hdr.Height).
			Hex("blockID", hdr.ID.Bytes()).
			Dur("timeSinceLastBlockInMS", time.Since(lastBlockTime)).
			Dur("timeToParseBlockInMS", time.Since(blockResolutionStart)).
			Dur("timeSinceBlockInMS", time.Since(hdr.Timestamp)).
			Uint64("txsTotal", totalStats.txs).
			Uint64("txsTotalUnknown", totalStats.unknownTxs).
			Uint64("txsTotalErrors", totalStats.errorTxs).
			Uint64("txsInBlock", blockStats.txs).
			Uint64("txsInBlockUnknown", blockStats.unknownTxs).
			Uint64("txsInBlockErrors", blockStats.errorTxs).
			Int("txsInProgress", f.InProgress()).
			Msg("new block parsed")

		f.updateFromBlockHeader(*hdr)

		lastBlockTime = time.Now()
	}
}

func (f *txFollowerImpl) getNextBlocksTransactions() (*flowsdk.BlockHeader, []*flowsdk.TransactionResult, error) {
	ctx, cancel := context.WithTimeout(f.ctx, 1*time.Second)
	defer cancel()

	nextHeight := f.Height() + 1
	hdr, err := f.client.GetBlockHeaderByHeight(ctx, nextHeight)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get latest block header: %w", err)
	}

	results, err := f.client.GetTransactionResultsByBlockID(ctx, hdr.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get block by height: %w", err)
	}

	return hdr, results, nil
}

// Follow returns a channel that will be closed when the transaction is completed.
// If transaction is already being followed, return the existing channel.
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
