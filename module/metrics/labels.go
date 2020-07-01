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
	EngineProposal               = "proposal"
	EngineCollectionIngest       = "collection_ingest"
	EngineCollectionProvider     = "collection_provider"
	EngineClusterSynchronization = "cluster-sync"
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
	ResourceUndefined                 = "undefined"
	ResourceProposal                  = "proposal"
	ResourceHeader                    = "header"
	ResourceIndex                     = "index"
	ResourceIdentity                  = "identity"
	ResourceGuarantee                 = "guarantee"
	ResourceResult                    = "result"
	ResourceReceipt                   = "receipt"
	ResourceCollection                = "collection"
	ResourceApproval                  = "approval"
	ResourceSeal                      = "seal"
	ResourceCommit                    = "commit"
	ResourceTransaction               = "transaction"
	ResourceClusterPayload            = "cluster_payload"
	ResourceClusterProposal           = "cluster_proposal"
	ResourceProcessedResultID         = "processed_result_ids"          // verification node, finder engine
	ResourcePendingReceipt            = "pending_receipt"               // verification node, finder engine
	ResourcePendingReceiptIDsByResult = "pending_receipt_ids_by_result" // verification node, finder engine
	ResourcePendingReceiptIDsByBlock  = "pending_receipt_ids_by_block"  // verification node, finder engine
	ResourcePendingResult             = "pending_result"                // verification node, match engine
	ResourcePendingChunk              = "pending_chunk"                 // verification node, match engine
	ResourcePendingBlock              = "pending_block"                 // verification node, match engine
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
	MessageClusterBlockResponse = "cluster_block_response"
	MessageSyncedClusterBlock   = "synced_cluster_block"
	MessageTransaction          = "transaction"
	MessageSubmitGuarantee      = "submit_guarantee"
	MessageCollectionRequest    = "collection_request"
	MessageCollectionResponse   = "collection_response"
)
