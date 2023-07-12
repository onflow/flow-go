package internal

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/common/worker"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/mempool/queue"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

// trackRpcSentWork is an internal data structure for "temporarily" storing *pubsub.RPC sent in the queue before they are processed
// by the *RPCSentTracker.
type trackRpcSentWork struct {
	rpc *pubsub.RPC
}

// RPCSentTracker tracks RPC messages that are sent.
type RPCSentTracker struct {
	component.Component
	cache      *rpcSentCache
	workerPool *worker.Pool[trackRpcSentWork]
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

	tracker := &RPCSentTracker{cache: newRPCSentCache(cacheConfig)}
	tracker.workerPool = worker.NewWorkerPoolBuilder[trackRpcSentWork](
		config.Logger,
		store,
		tracker.rpcSent).Build()

	builder := component.NewComponentManagerBuilder()
	for i := 0; i < config.NumOfWorkers; i++ {
		builder.AddWorker(tracker.workerPool.WorkerLogic())
	}
	tracker.Component = builder.Build()

	return tracker
}

// RPCSent submits the control message to the worker queue for async tracking.
// Args:
// - *pubsub.RPC: the rpc sent.
func (t *RPCSentTracker) RPCSent(rpc *pubsub.RPC) {
	t.workerPool.Submit(trackRpcSentWork{rpc})
}

// rpcSent tracks control messages sent in *pubsub.RPC.
func (t *RPCSentTracker) rpcSent(work trackRpcSentWork) error {
	switch {
	case len(work.rpc.GetControl().GetIhave()) > 0:
		t.iHaveRPCSent(work.rpc.GetControl().GetIhave())
	}
	return nil
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
