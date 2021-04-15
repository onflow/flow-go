package metrics

const (
	LabelChannel     = "topic"
	LabelChain       = "chain"
	LabelProposer    = "proposer"
	EngineLabel      = "engine"
	LabelResource    = "resource"
	LabelMessage     = "message"
	LabelNodeID      = "nodeid"
	LabelNodeAddress = "nodeaddress"
	LabelNodeRole    = "noderole"
	LabelNodeInfo    = "nodeinfo"
	LabelPriority    = "priority"
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
	EngineSealing            = "sealing"
	EngineSynchronization    = "sync"
	// common
	EngineFollower = "follower"
)

const (
	ResourceUndefined                = "undefined"
	ResourceProposal                 = "proposal"
	ResourceHeader                   = "header"
	ResourceFinalizedHeight          = "finalized_height"
	ResourceIndex                    = "index"
	ResourceIdentity                 = "identity"
	ResourceGuarantee                = "guarantee"
	ResourceResult                   = "result"
	ResourceResultApprovals          = "result_approvals"
	ResourceReceipt                  = "receipt"
	ResourceMyReceipt                = "my_receipt"
	ResourceCollection               = "collection"
	ResourceApproval                 = "approval"
	ResourceSeal                     = "seal"
	ResourceCommit                   = "commit"
	ResourceTransaction              = "transaction"
	ResourceClusterPayload           = "cluster_payload"
	ResourceClusterProposal          = "cluster_proposal"
	ResourceProcessedResultID        = "processed_result_id"          // verification node, finder engine
	ResourceDiscardedResultID        = "discarded_result_id"          // verification node, finder engine
	ResourcePendingReceipt           = "pending_receipt"              // verification node, finder engine
	ResourceReceiptIDsByResult       = "receipt_ids_by_result"        // verification node, finder engine
	ResourcePendingReceiptIDsByBlock = "pending_receipt_ids_by_block" // verification node, finder engine
	ResourcePendingResult            = "pending_result"               // verification node, match engine
	ResourceChunkIDsByResult         = "chunk_ids_by_result"          // verification node, match engine
	ResourcePendingChunk             = "pending_chunk"                // verification node, match engine
	ResourcePendingBlock             = "pending_block"                // verification node, match engine
	ResourceCachedReceipt            = "cached_receipt"               // verification node, finder engine
	ResourceCachedBlockID            = "cached_block_id"              // verification node, finder engine
	ResourceEpochSetup               = "epoch_setup"
	ResourceEpochCommit              = "epoch_commit"
	ResourceEpochStatus              = "epoch_status"
	ResourceApprovalQueue            = "sealing_approval_queue"          // consensus node, sealing engine
	ResourceReceiptQueue             = "sealing_receipt_queue"           // consensus node, sealing engine
	ResourceApprovalResponseQueue    = "sealing_approval_response_queue" // consensus node, sealing engine
	ResourceBlockProposalQueue       = "compliance_proposal_queue"       // consensus node, compliance engine
	ResourceBlockVoteQueue           = "compliance_vote_queue"           // consensus node, compliance engine
	ResourceChunkDataPack            = "chunk_data_pack"                 // execution node
	ResourceEvents                   = "events"                          // execution node
	ResourceServiceEvents            = "service_events"                  // execution node
	ResourceTransactionResults       = "transaction_results"             // execution node
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
	MessageEntityRequest        = "entity_request"
	MessageEntityResponse       = "entity_response"
)
