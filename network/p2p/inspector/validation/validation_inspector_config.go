package validation

const (
	// DefaultNumberOfWorkers default number of workers for the inspector component.
	DefaultNumberOfWorkers = 5
	// DefaultControlMsgValidationInspectorQueueCacheSize is the default size of the inspect message queue.
	DefaultControlMsgValidationInspectorQueueCacheSize = 100
	// DefaultClusterPrefixedControlMsgsReceivedCacheSize is the default size of the cluster prefixed topics received record cache.
	DefaultClusterPrefixedControlMsgsReceivedCacheSize = 150
	// DefaultClusterPrefixedControlMsgsReceivedCacheDecay the default cache decay value for cluster prefixed topics received cached counters.
	DefaultClusterPrefixedControlMsgsReceivedCacheDecay = 0.99
	// rpcInspectorComponentName the rpc inspector component name.
	rpcInspectorComponentName = "gossipsub_rpc_validation_inspector"
)
