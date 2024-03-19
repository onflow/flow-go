package metrics

// Prometheus metric namespaces
const (
	namespaceNetwork           = "network"
	namespaceStorage           = "storage"
	namespaceAccess            = "access"
	namespaceObserver          = "observer"
	namespaceCollection        = "collection"
	namespaceConsensus         = "consensus"
	namespaceVerification      = "verification"
	namespaceExecution         = "execution"
	namespaceLoader            = "loader"
	namespaceStateSync         = "state_synchronization"
	namespaceExecutionDataSync = "execution_data_sync"
	namespaceChainsync         = "chainsync"
	namespaceFollowerEngine    = "follower"
	namespaceRestAPI           = "access_rest_api"
	namespaceMachineAcct       = "machine_account"
)

// Network subsystems represent the various layers of networking.
const (
	subsystemLibp2p       = "libp2p"
	subsystemGossip       = "gossip"
	subsystemEngine       = "engine"
	subsystemQueue        = "queue"
	subsystemDHT          = "dht"
	subsystemBitswap      = "bitswap"
	subsystemAuth         = "authorization"
	subsystemRateLimiting = "ratelimit"
	subsystemAlsp         = "alsp"
	subsystemSecurity     = "security"
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
	subsystemConnectionPool        = "connection_pool"
	subsystemHTTP                  = "http"
)

// Observer subsystem
const (
	subsystemObserverGRPC = "observer_grpc"
)

// Collection subsystem
const (
	subsystemProposal = "proposal"
)

// Consensus subsystems represent the different components of the consensus algorithm.
const (
	subsystemCompliance  = "compliance"
	subsystemHotstuff    = "hotstuff"
	subsystemCruiseCtl   = "cruisectl"
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

// Execution Data Sync Subsystems
const (
	subsystemExeDataRequester       = "requester"
	subsystemExeDataProvider        = "provider"
	subsystemExeDataPruner          = "pruner"
	subsystemExecutionDataRequester = "execution_data_requester"
	subsystemExecutionStateIndexer  = "execution_state_indexer"
	subsystemExeDataBlobstore       = "blobstore"
)

// module/synchronization core
const (
	subsystemSyncCore = "sync_core"
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
