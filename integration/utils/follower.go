package utils

import (
	"context"
	"sync"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"

	"github.com/rs/zerolog"
)

type TxFollower interface {
	// CompleteChanByID returns a channel that is closed when the transaction is complete.
	CompleteChanByID(ID flowsdk.Identifier) <-chan struct{}

	// Height returns the last acted upon block height.
	Height() uint64
	// BlockID returns the last acted upon block ID.
	BlockID() flowsdk.Identifier

	// Stop sends termination signal to the follower.
	// It does not wait for the it to stop.
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

type txFollowerImpl struct {
	logger zerolog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	client *client.Client

	interval time.Duration

	mu      *sync.RWMutex
	height  uint64
	blockID flowsdk.Identifier

	txToChan sync.Map
}

type txInfo struct {
	submisionTime time.Time

	C chan struct{}
}

// NewTxFollower creates a new follower that tracks the current block height
// and can notify on transaction completion.
func NewTxFollower(ctx context.Context, client *client.Client, opts ...followerOption) (TxFollower, error) {
	newCtx, cancel := context.WithCancel(ctx)

	f := &txFollowerImpl{
		client: client,
		ctx:    newCtx,
		cancel: cancel,
		logger: zerolog.Nop(),
		mu:     &sync.RWMutex{},

		interval: 500 * time.Millisecond,
	}

	for _, opt := range opts {
		opt(f)
	}

	if f.height == 0 {
		hdr, err := client.GetLatestBlockHeader(newCtx, true)
		if err != nil {
			return nil, err
		}
		f.height = hdr.Height
		f.blockID = hdr.ID
	}

	go f.follow()

	return f, nil
}

func (f *txFollowerImpl) follow() {
	t := time.NewTicker(f.interval)
	defer t.Stop()

Loop:
	for lastBlockTime := time.Now(); ; <-t.C {
		select {
		case <-f.ctx.Done():
			return
		default:
		}

		block, err := f.client.GetBlockByHeight(f.ctx, f.height+1)
		if err != nil {
			continue
		}

		for _, guaranteed := range block.CollectionGuarantees[:] {
			col, err := f.client.GetCollection(f.ctx, guaranteed.CollectionID)
			if err != nil {
				continue Loop
			}
			for _, tx := range col.TransactionIDs {
				if ch, loaded := f.txToChan.LoadAndDelete(tx.Hex()); loaded {
					txi := ch.(txInfo)

					f.logger.Trace().
						Dur("duration", time.Since(txi.submisionTime)).
						Hex("txID", tx.Bytes()).
						Msg("returned tx to the pool")
					close(txi.C)
				}
			}
		}

		f.logger.Debug().
			Hex("blockID", block.ID.Bytes()).
			Dur("duration", time.Since(lastBlockTime)).
			Uint64("height", block.Height).
			Int("numCollections", len(block.CollectionGuarantees[:])).
			Int("numSeals", len(block.Seals)).
			Msg("new block parsed")

		f.mu.Lock()
		f.height = block.Height
		f.blockID = block.ID
		f.mu.Unlock()

		lastBlockTime = time.Now()
	}
}

func (f *txFollowerImpl) CompleteChanByID(ID flowsdk.Identifier) <-chan struct{} {
	txi, _ := f.txToChan.LoadOrStore(ID.Hex(), txInfo{submisionTime: time.Now(), C: make(chan struct{})})
	return txi.(txInfo).C
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
}

type nopTxFollower struct {
	txFollowerImpl

	closedCh chan struct{}
}

// NewNopTxFollower creates a new follower that tracks the current block height and ID but does not notify on transaction completion.
func NewNopTxFollower(ctx context.Context, client *client.Client, opts ...followerOption) (TxFollower, error) {
	f, err := NewTxFollower(ctx, client, opts...)
	if err != nil {
		return nil, err
	}
	impl, _ := f.(*txFollowerImpl)

	closedCh := make(chan struct{})
	close(closedCh)

	nop := &nopTxFollower{
		txFollowerImpl: *impl,
		closedCh:       closedCh,
	}
	return nop, nil
}

// CompleteChanByID always returns a closed channel.
func (nop *nopTxFollower) CompleteChanByID(ID flowsdk.Identifier) <-chan struct{} {
	return nop.closedCh
}
