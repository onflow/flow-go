package metrics

// Prometheus metric namespaces
const (
	namespaceNetwork      = "network"
	namespaceStorage      = "storage"
	namespaceAccess       = "access"
	namespaceCollection   = "collection"
	namespaceConsensus    = "consensus"
	namespaceVerification = "verification"
	namespaceExecution    = "execution"
	namespaceLoader       = "loader"
	namespaceStateSync    = "state_synchronization"
)

// Network subsystems represent the various layers of networking.
const (
	// subsystemLibp2p = "libp2p"
	subsystemGossip = "gossip"
	subsystemEngine = "engine"
	subsystemQueue  = "queue"
)

// Storage subsystems represent the various components of the storage layer.
const (
	subsystemBadger  = "badger"
	subsystemMempool = "mempool"
	subsystemCache   = "cache"
)

// Access subsystem
const (
	subsystemTransactionTiming     = "transaction_timing"
	subsystemTransactionSubmission = "transaction_submission"
)

// Collection subsystem
const (
	subsystemProposal = "proposal"
)

// Consensus subsystems represent the different components of the consensus algorithm.
const (
	subsystemCompliance  = "compliance"
	subsystemHotstuff    = "hotstuff"
	subsystemMatchEngine = "match"
)

// Execution Subsystems
const (
	subsystemStateStorage      = "state_storage"
	subsystemMTrie             = "mtrie"
	subsystemIngestion         = "ingestion"
	subsystemRuntime           = "runtime"
	subsystemProvider          = "provider"
	subsystemBlockDataUploader = "block_data_uploader"
)

// Verification Subsystems
const (
	subsystemAssignerEngine  = "assigner"
	subsystemFetcherEngine   = "fetcher"
	subsystemRequesterEngine = "requester"
	subsystemVerifierEngine  = "verifier"
	subsystemBlockConsumer   = "block_consumer"
	subsystemChunkConsumer   = "chunk_consumer"
)

// State Synchronization Subsystems
const (
	subsystemExecutionDataService = "execution_data_service"
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
