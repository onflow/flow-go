package inspector

import (
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/common/worker"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/internal"
)

const (
	// DefaultControlMsgMetricsInspectorNumberOfWorkers default number of workers for the inspector component.
	DefaultControlMsgMetricsInspectorNumberOfWorkers = 1
	// DefaultControlMsgMetricsInspectorQueueCacheSize is the default size of the message queue.
	DefaultControlMsgMetricsInspectorQueueCacheSize = 100
	// rpcInspectorComponentName the rpc inspector component name.
	rpcInspectorComponentName = "gossipsub_rpc_metrics_observer_inspector"
)

// ObserveRPCMetricsRequest represents a request to capture metrics for the provided RPC
type ObserveRPCMetricsRequest struct {
	// Nonce adds random value so that when msg req is stored on hero store a unique ID can be created from the struct fields.
	Nonce []byte
	// From the sender of the RPC.
	From peer.ID
	// rpc the rpc message.
	rpc *pubsub.RPC
}

// ControlMsgMetricsInspector a  GossipSub RPC inspector that will observe incoming RPC's and collect metrics related to control messages.
type ControlMsgMetricsInspector struct {
	component.Component
	logger zerolog.Logger
	// NumberOfWorkers number of component workers.
	NumberOfWorkers int
	// workerPool queue that stores *ObserveRPCMetricsRequest that will be processed by component workers.
	workerPool *worker.Pool[*ObserveRPCMetricsRequest]
	metrics    p2p.GossipSubControlMetricsObserver
}

var _ p2p.GossipSubRPCInspector = (*ControlMsgMetricsInspector)(nil)

// Inspect submits a request to the worker pool to observe metrics for the rpc.
// All errors returned from this function can be considered benign.
func (c *ControlMsgMetricsInspector) Inspect(from peer.ID, rpc *pubsub.RPC) error {
	nonce, err := internal.Nonce()
	if err != nil {
		return fmt.Errorf("failed to get observe rpc metrics request nonce: %w", err)
	}
	c.workerPool.Submit(&ObserveRPCMetricsRequest{Nonce: nonce, From: from, rpc: rpc})
	return nil
}

// ObserveRPC collects metrics for the rpc.
// No error is ever returned from this func.
func (c *ControlMsgMetricsInspector) ObserveRPC(req *ObserveRPCMetricsRequest) error {
	c.metrics.ObserveRPC(req.From, req.rpc)
	return nil
}

// Name returns the name of the rpc inspector.
func (c *ControlMsgMetricsInspector) Name() string {
	return rpcInspectorComponentName
}

// NewControlMsgMetricsInspector returns a new *ControlMsgMetricsInspector
func NewControlMsgMetricsInspector(logger zerolog.Logger, metricsObserver p2p.GossipSubControlMetricsObserver, numberOfWorkers int, heroStoreOpts ...queue.HeroStoreConfigOption) *ControlMsgMetricsInspector {
	lg := logger.With().Str("component", "gossip_sub_rpc_metrics_observer_inspector").Logger()
	c := &ControlMsgMetricsInspector{
		logger:          lg,
		NumberOfWorkers: numberOfWorkers,
		metrics:         metricsObserver,
	}

	cfg := &queue.HeroStoreConfig{
		SizeLimit: DefaultControlMsgMetricsInspectorQueueCacheSize,
		Collector: metrics.NewNoopCollector(),
	}

	for _, opt := range heroStoreOpts {
		opt(cfg)
	}
	store := queue.NewHeroStore(cfg.SizeLimit, logger, cfg.Collector)
	pool := worker.NewWorkerPoolBuilder[*ObserveRPCMetricsRequest](c.logger, store, c.ObserveRPC).Build()
	c.workerPool = pool

	builder := component.NewComponentManagerBuilder()
	for i := 0; i < c.NumberOfWorkers; i++ {
		builder.AddWorker(pool.WorkerLogic())
	}
	c.Component = builder.Build()

	return c
}
