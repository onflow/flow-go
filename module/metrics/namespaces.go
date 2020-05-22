package metrics

// Prometheus metric namespaces
const (
	namespaceNetwork      = "network"
	namespaceStorage      = "storage"
	namespaceCollection   = "collection"
	namespaceConsensus    = "consensus"
	namespaceVerification = "verification"
	namespaceExecution    = "execution"
)

// Network subsystems represent the various layers of networking.
const (
	// subsystemLibp2p = "libp2p"
	subsystemGossip = "gossip"
	subsystemEngine = "engine"
)

// Storage subsystems represent the various components of the storage layer.
const (
	subsystemBadger  = "badger"
	subsystemMempool = "mempool"
	subsystemCache   = "cache"
)

// Collection subsystem
const (
	subsystemProposal = "proposal"
)

// Consensus subsystems represent the different components of the consensus algorithm.
const (
	subsystemCompliance = "compliance"
	subsystemHotstuff   = "hotstuff"
)

// Execution Subsystems
const (
	subsystemStateStorage = "state_storage"
	subsystemMTrie        = "mtrie"
	subsystemIngestion    = "ingestion"
	subsystemRuntime      = "runtime"
)

// METRIC NAMING GUIDELINES
// Namespace:
//   * If it's under a module, use the module name. eg: hotstuff, network, storage, mempool, interpreter, crypto
//   * If it's a core metric from a node, use the node type. eg: consensus, verification, access
// Subsystem:
//   * Subsystem is optional if the entire namespace is small enough to not be segmented further.
//   * Within the component, describe the part or function referred to.
// Constant Labels:
//    * node_role: [collection, consensus, execution, verification, access]
//    * beta_metric: true
