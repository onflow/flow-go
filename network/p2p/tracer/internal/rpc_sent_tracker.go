package internal

import (
	"crypto/rand"
	"fmt"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/common/worker"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/queue"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

// trackRPC is an internal data structure for "temporarily" storing *pubsub.RPC sent in the queue before they are processed
// by the *RPCSentTracker.
type trackRPC struct {
	// Nonce prevents deduplication in the hero store
	Nonce []byte
	rpc   *pubsub.RPC
}

// RPCSentTracker tracks RPC messages that are sent.
type RPCSentTracker struct {
	component.Component
	cache      *rpcSentCache
	workerPool *worker.Pool[trackRPC]
	// lastHighestIHaveRPCSize tracks the size of the last largest iHave rpc control message sent.
	lastHighestIHaveRPCSize              *atomic.Int64
	lastHighestIHaveRPCSizeResetInterval time.Duration
}

// RPCSentTrackerConfig configuration for the RPCSentTracker.
type RPCSentTrackerConfig struct {
	Logger zerolog.Logger
	//RPCSentCacheSize size of the *rpcSentCache cache.
	RPCSentCacheSize uint32
	// RPCSentCacheCollector metrics collector for the *rpcSentCache cache.
	RPCSentCacheCollector module.HeroCacheMetrics
	// WorkerQueueCacheCollector metrics factory for the worker pool.
	WorkerQueueCacheCollector module.HeroCacheMetrics
	// WorkerQueueCacheSize the worker pool herostore cache size.
	WorkerQueueCacheSize uint32
	// NumOfWorkers number of workers in the worker pool.
	NumOfWorkers int
	// LastHighestIhavesSentResetInterval the refresh interval to reset the lastHighestIHaveRPCSize.
	LastHighestIhavesSentResetInterval time.Duration
}

// NewRPCSentTracker returns a new *NewRPCSentTracker.
func NewRPCSentTracker(config *RPCSentTrackerConfig) *RPCSentTracker {
	cacheConfig := &rpcCtrlMsgSentCacheConfig{
		sizeLimit: config.RPCSentCacheSize,
		logger:    config.Logger,
		collector: config.RPCSentCacheCollector,
	}

	store := queue.NewHeroStore(
		config.WorkerQueueCacheSize,
		config.Logger,
		config.WorkerQueueCacheCollector)

	tracker := &RPCSentTracker{
		cache:                                newRPCSentCache(cacheConfig),
		lastHighestIHaveRPCSize:              atomic.NewInt64(0),
		lastHighestIHaveRPCSizeResetInterval: config.LastHighestIhavesSentResetInterval,
	}
	tracker.workerPool = worker.NewWorkerPoolBuilder[trackRPC](
		config.Logger,
		store,
		tracker.rpcSent).Build()

	tracker.Component = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			tracker.lastHighestIHaveRPCSizeResetLoop(ctx)
		}).
		AddWorkers(config.NumOfWorkers, tracker.workerPool.WorkerLogic()).
		Build()

	return tracker
}

// RPCSent submits the control message to the worker queue for async tracking.
// Args:
// - *pubsub.RPC: the rpc sent.
func (t *RPCSentTracker) RPCSent(rpc *pubsub.RPC) error {
	n, err := nonce()
	if err != nil {
		return fmt.Errorf("failed to get track rpc work nonce: %w", err)
	}
	t.workerPool.Submit(trackRPC{Nonce: n, rpc: rpc})
	return nil
}

// rpcSent tracks control messages sent in *pubsub.RPC.
func (t *RPCSentTracker) rpcSent(work trackRPC) error {
	switch {
	case len(work.rpc.GetControl().GetIhave()) > 0:
		iHave := work.rpc.GetControl().GetIhave()
		t.iHaveRPCSent(iHave)
		t.updateLastHighestIHaveRPCSize(int64(len(iHave)))
	}
	return nil
}

func (t *RPCSentTracker) updateLastHighestIHaveRPCSize(size int64) {
	if t.lastHighestIHaveRPCSize.Load() < size {
		t.lastHighestIHaveRPCSize.Store(size)
	}
}

// iHaveRPCSent caches a unique entity message ID for each message ID included in each rpc iHave control message.
// Args:
// - []*pb.ControlIHave: list of iHave control messages.
func (t *RPCSentTracker) iHaveRPCSent(iHaves []*pb.ControlIHave) {
	controlMsgType := p2pmsg.CtrlMsgIHave
	for _, iHave := range iHaves {
		topicID := iHave.GetTopicID()
		for _, messageID := range iHave.GetMessageIDs() {
			t.cache.add(topicID, messageID, controlMsgType)
		}
	}
}

// WasIHaveRPCSent checks if an iHave control message with the provided message ID was sent.
// Args:
// - string: the topic ID of the iHave RPC.
// - string: the message ID of the iHave RPC.
// Returns:
// - bool: true if the iHave rpc with the provided message ID was sent.
func (t *RPCSentTracker) WasIHaveRPCSent(topicID, messageID string) bool {
	return t.cache.has(topicID, messageID, p2pmsg.CtrlMsgIHave)
}

// LastHighestIHaveRPCSize returns the last highest size of iHaves sent in an rpc.
func (t *RPCSentTracker) LastHighestIHaveRPCSize() int64 {
	return t.lastHighestIHaveRPCSize.Load()
}

// lastHighestIHaveRPCSizeResetLoop resets the lastHighestIHaveRPCSize to 0 on each interval tick.
func (t *RPCSentTracker) lastHighestIHaveRPCSizeResetLoop(ctx irrecoverable.SignalerContext) {
	ticker := time.NewTicker(t.lastHighestIHaveRPCSizeResetInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.lastHighestIHaveRPCSize.Store(0)
		}
	}
}

// nonce returns random string that is used to store unique items in herocache.
func nonce() ([]byte, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
