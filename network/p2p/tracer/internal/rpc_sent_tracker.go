package internal

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/common/worker"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/mempool/queue"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

const (
	iHaveRPCTrackedLog = "ihave rpc tracked successfully"
)

// trackableRPC is an internal data structure for "temporarily" storing *pubsub.RPC sent in the queue before they are processed
// by the *RPCSentTracker.
type trackableRPC struct {
	// Nonce prevents deduplication in the hero store
	Nonce []byte
	rpc   *pubsub.RPC
}

// lastHighestIHaveRPCSize tracks the last highest rpc control message size the time stamp it was last updated.
type lastHighestIHaveRPCSize struct {
	sync.RWMutex
	lastSize   int64
	lastUpdate time.Time
}

// RPCSentTracker tracks RPC messages and the size of the last largest iHave rpc control message sent.
type RPCSentTracker struct {
	component.Component
	*lastHighestIHaveRPCSize
	logger                               zerolog.Logger
	cache                                *rpcSentCache
	workerPool                           *worker.Pool[trackableRPC]
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
		logger:                               config.Logger.With().Str("component", "rpc_sent_tracker").Logger(),
		lastHighestIHaveRPCSize:              &lastHighestIHaveRPCSize{sync.RWMutex{}, 0, time.Now()},
		cache:                                newRPCSentCache(cacheConfig),
		lastHighestIHaveRPCSizeResetInterval: config.LastHighestIhavesSentResetInterval,
	}
	tracker.workerPool = worker.NewWorkerPoolBuilder[trackableRPC](
		config.Logger,
		store,
		tracker.rpcSentWorkerLogic).Build()

	builder := component.NewComponentManagerBuilder()
	for i := 0; i < config.NumOfWorkers; i++ {
		builder.AddWorker(tracker.workerPool.WorkerLogic())
	}
	tracker.Component = builder.Build()
	return tracker
}

// Track submits the control message to the worker queue for async tracking.
// Args:
// - *pubsub.RPC: the rpc sent.
// All errors returned from this function can be considered benign.
func (t *RPCSentTracker) Track(rpc *pubsub.RPC) error {
	n, err := nonce()
	if err != nil {
		return fmt.Errorf("failed to get track rpc work nonce: %w", err)
	}
	if ok := t.workerPool.Submit(trackableRPC{Nonce: n, rpc: rpc}); !ok {
		return fmt.Errorf("failed to track RPC could not submit work to worker pool")
	}
	return nil
}

// rpcSentWorkerLogic tracks control messages sent in *pubsub.RPC.
func (t *RPCSentTracker) rpcSentWorkerLogic(work trackableRPC) error {
	switch {
	case len(work.rpc.GetControl().GetIhave()) > 0:
		iHave := work.rpc.GetControl().GetIhave()
		t.iHaveRPCSent(iHave)
		t.updateLastHighestIHaveRPCSize(int64(len(iHave)))
		t.logger.Info().Int("size", len(iHave)).Msg(iHaveRPCTrackedLog)
	}
	return nil
}

func (t *RPCSentTracker) updateLastHighestIHaveRPCSize(size int64) {
	t.Lock()
	defer t.Unlock()
	if t.lastSize < size || time.Since(t.lastUpdate) > t.lastHighestIHaveRPCSizeResetInterval {
		// The last highest ihave RPC size is updated if the new size is larger than the current size, or if the time elapsed since the last update surpasses the reset interval.
		t.lastSize = size
		t.lastUpdate = time.Now()
	}
}

// iHaveRPCSent caches a unique entity message ID for each message ID included in each rpc iHave control message.
// Args:
// - []*pb.ControlIHave: list of iHave control messages.
func (t *RPCSentTracker) iHaveRPCSent(iHaves []*pb.ControlIHave) {
	controlMsgType := p2pmsg.CtrlMsgIHave
	for _, iHave := range iHaves {
		for _, messageID := range iHave.GetMessageIDs() {
			t.cache.add(messageID, controlMsgType)
		}
	}
}

// WasIHaveRPCSent checks if an iHave control message with the provided message ID was sent.
// Args:
// - messageID: the message ID of the iHave RPC.
// Returns:
// - bool: true if the iHave rpc with the provided message ID was sent.
func (t *RPCSentTracker) WasIHaveRPCSent(messageID string) bool {
	return t.cache.has(messageID, p2pmsg.CtrlMsgIHave)
}

// LastHighestIHaveRPCSize returns the last highest size of iHaves sent in an rpc.
func (t *RPCSentTracker) LastHighestIHaveRPCSize() int64 {
	t.RLock()
	defer t.RUnlock()
	return t.lastSize
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
