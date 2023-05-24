package validation

import (
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/network/p2p"
)

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

// ControlMsgValidationInspectorConfig validation configuration for each type of RPC control message.
type ControlMsgValidationInspectorConfig struct {
	*ClusterPrefixedMessageConfig
	// NumberOfWorkers number of component workers to start for processing RPC messages.
	NumberOfWorkers int
	// InspectMsgStoreOpts options used to configure the underlying herocache message store.
	InspectMsgStoreOpts []queue.HeroStoreConfigOption
	// GraftValidationCfg validation configuration for GRAFT control messages.
	GraftValidationCfg *CtrlMsgValidationConfig
	// PruneValidationCfg validation configuration for PRUNE control messages.
	PruneValidationCfg *CtrlMsgValidationConfig
	// IHaveValidationCfg validation configuration for IHAVE control messages.
	IHaveValidationCfg *CtrlMsgValidationConfig
}

// ClusterPrefixedMessageConfig configuration values for cluster prefixed control message validation.
type ClusterPrefixedMessageConfig struct {
	// ClusterPrefixHardThreshold the upper bound on the amount of cluster prefixed control messages that will be processed
	// before a node starts to get penalized. This allows LN nodes to process some cluster prefixed control messages during startup
	// when the cluster ID's provider is set asynchronously. It also allows processing of some stale messages that may be sent by nodes
	// that fall behind in the protocol. After the amount of cluster prefixed control messages processed exceeds this threshold the node
	// will be pushed to the edge of the network mesh.
	ClusterPrefixHardThreshold float64
	// ClusterPrefixedControlMsgsReceivedCacheSize size of the cache used to track the amount of cluster prefixed topics received by peers.
	ClusterPrefixedControlMsgsReceivedCacheSize uint32
	// ClusterPrefixedControlMsgsReceivedCacheDecay decay val used for the geometric decay of cache counters used to keep track of cluster prefixed topics received by peers.
	ClusterPrefixedControlMsgsReceivedCacheDecay float64
}

// getCtrlMsgValidationConfig returns the CtrlMsgValidationConfig for the specified p2p.ControlMessageType.
func (conf *ControlMsgValidationInspectorConfig) getCtrlMsgValidationConfig(controlMsg p2p.ControlMessageType) (*CtrlMsgValidationConfig, bool) {
	switch controlMsg {
	case p2p.CtrlMsgGraft:
		return conf.GraftValidationCfg, true
	case p2p.CtrlMsgPrune:
		return conf.PruneValidationCfg, true
	case p2p.CtrlMsgIHave:
		return conf.IHaveValidationCfg, true
	default:
		return nil, false
	}
}

// allCtrlMsgValidationConfig returns all control message validation configs in a list.
func (conf *ControlMsgValidationInspectorConfig) allCtrlMsgValidationConfig() CtrlMsgValidationConfigs {
	return CtrlMsgValidationConfigs{conf.GraftValidationCfg, conf.PruneValidationCfg, conf.IHaveValidationCfg}
}
