package internal

// Prometheus metric namespaces
const (
	NamespaceNetwork           = "network"
	NamespaceStorage           = "storage"
	NamespaceAccess            = "access"
	NamespaceObserver          = "observer"
	NamespaceCollection        = "collection"
	NamespaceConsensus         = "consensus"
	NamespaceVerification      = "verification"
	NamespaceExecution         = "execution"
	NamespaceLoader            = "loader"
	NamespaceStateSync         = "state_synchronization"
	NamespaceExecutionDataSync = "execution_data_sync"
	NamespaceChainsync         = "chainsync"
	NamespaceFollowerEngine    = "follower"
	NamespaceRestAPI           = "access_rest_api"
)

// Network subsystems represent the various layers of networking.
const (
	SubsystemLibp2p       = "libp2p"
	SubsystemGossip       = "gossip"
	SubsystemEngine       = "engine"
	SubsystemQueue        = "queue"
	SubsystemDHT          = "dht"
	SubsystemBitswap      = "bitswap"
	SubsystemAuth         = "authorization"
	SubsystemRateLimiting = "ratelimit"
	SubsystemAlsp         = "alsp"
	SubsystemSecurity     = "security"
)

// Storage subsystems represent the various components of the storage layer.
const (
	SubsystemBadger  = "badger"
	SubsystemMempool = "mempool"
	SubsystemCache   = "cache"
)

// Access subsystem
const (
	SubsystemTransactionTiming     = "transaction_timing"
	SubsystemTransactionSubmission = "transaction_submission"
	SubsystemConnectionPool        = "connection_pool"
	SubsystemHTTP                  = "http"
)

// Observer subsystem
const (
	SubsystemObserverGRPC = "observer_grpc"
)

// Collection subsystem
const (
	SubsystemProposal = "proposal"
)

// Consensus subsystems represent the different components of the consensus algorithm.
const (
	SubsystemCompliance  = "compliance"
	SubsystemHotstuff    = "hotstuff"
	SubsystemCruiseCtl   = "cruisectl"
	SubsystemMatchEngine = "match"
)

// Execution Subsystems
const (
	SubsystemStateStorage      = "state_storage"
	SubsystemMTrie             = "mtrie"
	SubsystemIngestion         = "ingestion"
	SubsystemRuntime           = "runtime"
	SubsystemProvider          = "provider"
	SubsystemBlockDataUploader = "block_data_uploader"
)

// Verification Subsystems
const (
	SubsystemAssignerEngine  = "assigner"
	SubsystemFetcherEngine   = "fetcher"
	SubsystemRequesterEngine = "requester"
	SubsystemVerifierEngine  = "verifier"
	SubsystemBlockConsumer   = "block_consumer"
	SubsystemChunkConsumer   = "chunk_consumer"
)

// Execution Data Sync Subsystems
const (
	SubsystemExeDataRequester       = "requester"
	SubsystemExeDataProvider        = "provider"
	SubsystemExeDataPruner          = "pruner"
	SubsystemExecutionDataRequester = "execution_data_requester"
	SubsystemExeDataBlobstore       = "blobstore"
)

// module/synchronization core
const (
	SubsystemSyncCore = "sync_core"
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
