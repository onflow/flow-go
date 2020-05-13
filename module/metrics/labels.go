package metrics

const (
	LabelChannel  = "topic"
	LabelChain    = "chain"
	EngineLabel   = "engine"
	LabelResource = "resource"
	LabelMessage  = "message"
)

const (
	ChannelOneToOne = "OneToOne"
)

const (
	// collection
	EngineProposal           = "proposal"
	EngineCollectionIngest   = "collection_ingest"
	EngineCollectionProvider = "collection_provider"
	// consensus
	EnginePropagation        = "propagation"
	EngineCompliance         = "compliance"
	EngineConsensusProvider  = "consensus_provider"
	EngineConsensusIngestion = "consensus_ingestion"
	EngineMatching           = "matching"
	EngineSynchronization    = "sync"
	// common
	EngineFollower = "follower"
)

const (
	ResourceUndefined = "undefined"
	ResourceProposal  = "proposal"
	ResourceHeader    = "header"
	ResourceIndex     = "index"
	ResourceIdentity  = "identity"
	ResourceGuarantee = "guarantee"
	ResourceResult    = "result"
	ResourceReceipt   = "receipt"
	ResourceApproval  = "approval"
	ResourceSeal      = "seal"
	ResourceCommit    = "commit"
)

const (
	MessageCollectionGuarantee  = "guarantee"
	MessageBlockProposal        = "proposal"
	MessageBlockVote            = "vote"
	MessageExecutionReceipt     = "receipt"
	MessageResultApproval       = "approval"
	MessageSyncRequest          = "ping"
	MessageSyncResponse         = "pong"
	MessageRangeRequest         = "range"
	MessageBatchRequest         = "batch"
	MessageBlockResponse        = "block"
	MessageSyncedBlock          = "synced_block"
	MessageClusterBlockProposal = "cluster_proposal"
	MessageClusterBlockVote     = "cluster_vote"
	MessageClusterBlockRequest  = "cluster_block_request"
	MessageClusterBlockResponse = "cluster_block_response"
	MessageTransaction          = "transaction"
	MessageSubmitGuarantee      = "submit_guarantee"
	MessageCollectionRequest    = "collection_request"
	MessageCollectionResponse   = "collection_response"
)
