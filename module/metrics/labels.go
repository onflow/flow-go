package metrics

const (
	LabelChannel             = "topic"
	LabelChain               = "chain"
	LabelProposer            = "proposer"
	EngineLabel              = "engine"
	LabelResource            = "resource"
	LabelProtocol            = "protocol"
	LabelMessage             = "message"
	LabelNodeID              = "nodeid"
	LabelNodeAddress         = "nodeaddress"
	LabelNodeRole            = "noderole"
	LabelNodeInfo            = "nodeinfo"
	LabelNodeVersion         = "nodeversion"
	LabelPriority            = "priority"
	LabelComputationKind     = "computationKind"
	LabelConnectionDirection = "direction"
	LabelConnectionUseFD     = "usefd" // whether the connection is using a file descriptor
	LabelSuccess             = "success"
	LabelMisbehavior         = "misbehavior"
	LabelHandler             = "handler"
	LabelStatusCode          = "code"
	LabelMethod              = "method"
	LabelService             = "service"
)

const (
	// collection
	EngineClusterCompliance      = "collection_compliance"
	EngineCollectionMessageHub   = "collection_message_hub"
	EngineCollectionIngest       = "collection_ingest"
	EngineCollectionProvider     = "collection_provider"
	EngineClusterSynchronization = "cluster-sync"
	// consensus
	EnginePropagation         = "propagation"
	EngineCompliance          = "compliance"
	EngineConsensusMessageHub = "consensus_message_hub"
	EngineConsensusIngestion  = "consensus_ingestion"
	EngineSealing             = "sealing"
	EngineSynchronization     = "sync"
	// common
	EngineFollower          = "follower"
	EngineVoteAggregator    = "vote_aggregator"
	EngineTimeoutAggregator = "timeout_aggregator"
)

const (
	ResourceUndefined                                  = "undefined"
	ResourceProposal                                   = "proposal"
	ResourceHeader                                     = "header"
	ResourceFinalizedHeight                            = "finalized_height"
	ResourceIndex                                      = "index"
	ResourceIdentity                                   = "identity"
	ResourceGuarantee                                  = "guarantee"
	ResourceResult                                     = "result"
	ResourceResultApprovals                            = "result_approvals"
	ResourceReceipt                                    = "receipt"
	ResourceQC                                         = "qc"
	ResourceMyReceipt                                  = "my_receipt"
	ResourceCollection                                 = "collection"
	ResourceApproval                                   = "approval"
	ResourceSeal                                       = "seal"
	ResourcePendingIncorporatedSeal                    = "pending_incorporated_seal"
	ResourceCommit                                     = "commit"
	ResourceTransaction                                = "transaction"
	ResourceClusterPayload                             = "cluster_payload"
	ResourceClusterProposal                            = "cluster_proposal"
	ResourceProcessedResultID                          = "processed_result_id"          // verification node, finder engine // TODO: remove finder engine labels
	ResourceDiscardedResultID                          = "discarded_result_id"          // verification node, finder engine
	ResourcePendingReceipt                             = "pending_receipt"              // verification node, finder engine
	ResourceReceiptIDsByResult                         = "receipt_ids_by_result"        // verification node, finder engine
	ResourcePendingReceiptIDsByBlock                   = "pending_receipt_ids_by_block" // verification node, finder engine
	ResourcePendingResult                              = "pending_result"               // verification node, match engine
	ResourceChunkIDsByResult                           = "chunk_ids_by_result"          // verification node, match engine
	ResourcePendingChunk                               = "pending_chunk"                // verification node, match engine
	ResourcePendingBlock                               = "pending_block"                // verification node, match engine
	ResourceCachedReceipt                              = "cached_receipt"               // verification node, finder engine
	ResourceCachedBlockID                              = "cached_block_id"              // verification node, finder engine
	ResourceChunkStatus                                = "chunk_status"                 // verification node, fetcher engine
	ResourceChunkRequest                               = "chunk_request"                // verification node, requester engine
	ResourceChunkConsumer                              = "chunk_consumer_jobs"          // verification node
	ResourceBlockConsumer                              = "block_consumer_jobs"          // verification node
	ResourceEpochSetup                                 = "epoch_setup"
	ResourceEpochCommit                                = "epoch_commit"
	ResourceEpochStatus                                = "epoch_status"
	ResourceNetworkingReceiveCache                     = "networking_received_message" // networking layer
	ResourceNetworkingSubscriptionRecordsCache         = "subscription_records_cache"  // networking layer
	ResourceNetworkingDnsIpCache                       = "networking_dns_ip_cache"     // networking layer
	ResourceNetworkingDnsTxtCache                      = "networking_dns_txt_cache"    // networking layer
	ResourceNetworkingDisallowListNotificationQueue    = "networking_disallow_list_notification_queue"
	ResourceNetworkingRpcInspectorNotificationQueue    = "networking_rpc_inspector_notification_queue"
	ResourceNetworkingRpcValidationInspectorQueue      = "networking_rpc_validation_inspector_queue"
	ResourceNetworkingRpcMetricsObserverInspectorQueue = "networking_rpc_metrics_observer_inspector_queue"
	ResourceNetworkingApplicationLayerSpamRecordCache  = "application_layer_spam_record_cache"
	ResourceNetworkingApplicationLayerSpamReportQueue  = "application_layer_spam_report_queue"
	ResourceNetworkingRpcClusterPrefixReceivedCache    = "rpc_cluster_prefixed_received_cache"
	ResourceNetworkingAppSpecificScoreUpdateQueue      = "gossipsub_app_specific_score_update_queue"
	ResourceNetworkingDisallowListCache                = "disallow_list_cache"
	ResourceNetworkingRPCSentTrackerCache              = "gossipsub_rpc_sent_tracker_cache"
	ResourceNetworkingRPCSentTrackerQueue              = "gossipsub_rpc_sent_tracker_queue"
	ResourceNetworkingUnicastDialConfigCache           = "unicast_dial_config_cache"

	ResourceFollowerPendingBlocksCache         = "follower_pending_block_cache"           // follower engine
	ResourceFollowerLoopCertifiedBlocksChannel = "follower_loop_certified_blocks_channel" // follower loop, certified blocks buffered channel
	ResourceClusterBlockProposalQueue          = "cluster_compliance_proposal_queue"      // collection node, compliance engine
	ResourceTransactionIngestQueue             = "ingest_transaction_queue"               // collection node, ingest engine
	ResourceBeaconKey                          = "beacon-key"                             // consensus node, DKG engine
	ResourceDKGMessage                         = "dkg_private_message"                    // consensus, DKG messaging engine
	ResourceApprovalQueue                      = "sealing_approval_queue"                 // consensus node, sealing engine
	ResourceReceiptQueue                       = "sealing_receipt_queue"                  // consensus node, sealing engine
	ResourceApprovalResponseQueue              = "sealing_approval_response_queue"        // consensus node, sealing engine
	ResourceBlockResponseQueue                 = "compliance_block_response_queue"        // consensus node, compliance engine
	ResourceBlockProposalQueue                 = "compliance_proposal_queue"              // consensus node, compliance engine
	ResourceBlockVoteQueue                     = "vote_aggregator_queue"                  // consensus/collection node, vote aggregator
	ResourceTimeoutObjectQueue                 = "timeout_aggregator_queue"               // consensus/collection node, timeout aggregator
	ResourceCollectionGuaranteesQueue          = "ingestion_col_guarantee_queue"          // consensus node, ingestion engine
	ResourceChunkDataPack                      = "chunk_data_pack"                        // execution node
	ResourceChunkDataPackRequests              = "chunk_data_pack_request"                // execution node
	ResourceEvents                             = "events"                                 // execution node
	ResourceServiceEvents                      = "service_events"                         // execution node
	ResourceTransactionResults                 = "transaction_results"                    // execution node
	ResourceTransactionResultIndices           = "transaction_result_indices"             // execution node
	ResourceTransactionResultByBlock           = "transaction_result_by_block"            // execution node
	ResourceExecutionDataCache                 = "execution_data_cache"                   // access node
)

const (
	MessageCollectionGuarantee = "guarantee"
	MessageBlockProposal       = "proposal"
	MessageBlockVote           = "vote"
	MessageTimeoutObject       = "timeout_object"
	MessageExecutionReceipt    = "receipt"
	MessageResultApproval      = "approval"
	MessageSyncRequest         = "ping"
	MessageSyncResponse        = "pong"
	MessageRangeRequest        = "range"
	MessageBatchRequest        = "batch"
	MessageBlockResponse       = "block"
	MessageSyncedBlocks        = "synced_blocks"
	MessageSyncedClusterBlock  = "synced_cluster_block"
	MessageTransaction         = "transaction"
	MessageSubmitGuarantee     = "submit_guarantee"
	MessageCollectionRequest   = "collection_request"
	MessageCollectionResponse  = "collection_response"
	MessageEntityRequest       = "entity_request"
	MessageEntityResponse      = "entity_response"
)

const ExecutionDataRequestRetryable = "retryable"

const LabelViolationReason = "reason"
const LabelRateLimitReason = "reason"
