package metrics

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	httpmetrics "github.com/slok/go-http-metrics/metrics"

	"github.com/onflow/flow-go/model/chainsync"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/channels"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

type NoopCollector struct{}

func NewNoopCollector() *NoopCollector {
	nc := &NoopCollector{}
	return nc
}

func (nc *NoopCollector) OutboundMessageSent(int, string, string, string)        {}
func (nc *NoopCollector) InboundMessageReceived(int, string, string, string)     {}
func (nc *NoopCollector) DuplicateInboundMessagesDropped(string, string, string) {}
func (nc *NoopCollector) UnicastMessageSendingStarted(topic string)              {}
func (nc *NoopCollector) UnicastMessageSendingCompleted(topic string)            {}
func (nc *NoopCollector) BlockProposed(*flow.Block)                              {}
func (nc *NoopCollector) BlockProposalDuration(duration time.Duration)           {}

// interface check
var _ module.BackendScriptsMetrics = (*NoopCollector)(nil)
var _ module.TransactionMetrics = (*NoopCollector)(nil)
var _ module.HotstuffMetrics = (*NoopCollector)(nil)
var _ module.EngineMetrics = (*NoopCollector)(nil)
var _ module.HeroCacheMetrics = (*NoopCollector)(nil)
var _ module.NetworkMetrics = (*NoopCollector)(nil)

func (nc *NoopCollector) Peers(prefix string, n int)                                             {}
func (nc *NoopCollector) Wantlist(prefix string, n int)                                          {}
func (nc *NoopCollector) BlobsReceived(prefix string, n uint64)                                  {}
func (nc *NoopCollector) DataReceived(prefix string, n uint64)                                   {}
func (nc *NoopCollector) BlobsSent(prefix string, n uint64)                                      {}
func (nc *NoopCollector) DataSent(prefix string, n uint64)                                       {}
func (nc *NoopCollector) DupBlobsReceived(prefix string, n uint64)                               {}
func (nc *NoopCollector) DupDataReceived(prefix string, n uint64)                                {}
func (nc *NoopCollector) MessagesReceived(prefix string, n uint64)                               {}
func (nc *NoopCollector) NetworkMessageSent(sizeBytes int, topic string, messageType string)     {}
func (nc *NoopCollector) NetworkMessageReceived(sizeBytes int, topic string, messageType string) {}
func (nc *NoopCollector) NetworkDuplicateMessagesDropped(topic string, messageType string)       {}
func (nc *NoopCollector) MessageAdded(priority int)                                              {}
func (nc *NoopCollector) MessageRemoved(priority int)                                            {}
func (nc *NoopCollector) QueueDuration(duration time.Duration, priority int)                     {}
func (nc *NoopCollector) MessageProcessingStarted(topic string)                                  {}
func (nc *NoopCollector) MessageProcessingFinished(topic string, duration time.Duration)         {}
func (nc *NoopCollector) DirectMessageStarted(topic string)                                      {}
func (nc *NoopCollector) DirectMessageFinished(topic string)                                     {}
func (nc *NoopCollector) MessageSent(engine string, message string)                              {}
func (nc *NoopCollector) MessageReceived(engine string, message string)                          {}
func (nc *NoopCollector) MessageHandled(engine string, message string)                           {}
func (nc *NoopCollector) InboundMessageDropped(engine string, message string)                    {}
func (nc *NoopCollector) OutboundMessageDropped(engine string, messages string)                  {}
func (nc *NoopCollector) OutboundConnections(_ uint)                                             {}
func (nc *NoopCollector) InboundConnections(_ uint)                                              {}
func (nc *NoopCollector) DNSLookupDuration(duration time.Duration)                               {}
func (nc *NoopCollector) OnDNSCacheMiss()                                                        {}
func (nc *NoopCollector) OnDNSCacheInvalidated()                                                 {}
func (nc *NoopCollector) OnDNSCacheHit()                                                         {}
func (nc *NoopCollector) OnDNSLookupRequestDropped()                                             {}
func (nc *NoopCollector) UnstakedOutboundConnections(_ uint)                                     {}
func (nc *NoopCollector) UnstakedInboundConnections(_ uint)                                      {}
func (nc *NoopCollector) RanGC(duration time.Duration)                                           {}
func (nc *NoopCollector) BadgerLSMSize(sizeBytes int64)                                          {}
func (nc *NoopCollector) BadgerVLogSize(sizeBytes int64)                                         {}
func (nc *NoopCollector) BadgerNumReads(n int64)                                                 {}
func (nc *NoopCollector) BadgerNumWrites(n int64)                                                {}
func (nc *NoopCollector) BadgerNumBytesRead(n int64)                                             {}
func (nc *NoopCollector) BadgerNumBytesWritten(n int64)                                          {}
func (nc *NoopCollector) BadgerNumGets(n int64)                                                  {}
func (nc *NoopCollector) BadgerNumPuts(n int64)                                                  {}
func (nc *NoopCollector) BadgerNumBlockedPuts(n int64)                                           {}
func (nc *NoopCollector) BadgerNumMemtableGets(n int64)                                          {}
func (nc *NoopCollector) FinalizedHeight(height uint64)                                          {}
func (nc *NoopCollector) SealedHeight(height uint64)                                             {}
func (nc *NoopCollector) BlockFinalized(*flow.Block)                                             {}
func (nc *NoopCollector) BlockSealed(*flow.Block)                                                {}
func (nc *NoopCollector) EpochTransitionHeight(height uint64)                                    {}
func (nc *NoopCollector) CurrentEpochCounter(counter uint64)                                     {}
func (nc *NoopCollector) CurrentEpochPhase(phase flow.EpochPhase)                                {}
func (nc *NoopCollector) CurrentEpochFinalView(view uint64)                                      {}
func (nc *NoopCollector) CurrentDKGPhase1FinalView(view uint64)                                  {}
func (nc *NoopCollector) CurrentDKGPhase2FinalView(view uint64)                                  {}
func (nc *NoopCollector) CurrentDKGPhase3FinalView(view uint64)                                  {}
func (nc *NoopCollector) EpochEmergencyFallbackTriggered()                                       {}
func (nc *NoopCollector) CacheEntries(resource string, entries uint)                             {}
func (nc *NoopCollector) CacheHit(resource string)                                               {}
func (nc *NoopCollector) CacheNotFound(resource string)                                          {}
func (nc *NoopCollector) CacheMiss(resource string)                                              {}
func (nc *NoopCollector) MempoolEntries(resource string, entries uint)                           {}
func (nc *NoopCollector) Register(resource string, entriesFunc module.EntriesFunc) error         { return nil }
func (nc *NoopCollector) HotStuffBusyDuration(duration time.Duration, event string)              {}
func (nc *NoopCollector) HotStuffIdleDuration(duration time.Duration)                            {}
func (nc *NoopCollector) HotStuffWaitDuration(duration time.Duration, event string)              {}
func (nc *NoopCollector) SetCurView(view uint64)                                                 {}
func (nc *NoopCollector) SetQCView(view uint64)                                                  {}
func (nc *NoopCollector) SetTCView(uint64)                                                       {}
func (nc *NoopCollector) CountSkipped()                                                          {}
func (nc *NoopCollector) CountTimeout()                                                          {}
func (nc *NoopCollector) BlockProcessingDuration(time.Duration)                                  {}
func (nc *NoopCollector) VoteProcessingDuration(time.Duration)                                   {}
func (nc *NoopCollector) TimeoutObjectProcessingDuration(time.Duration)                          {}
func (nc *NoopCollector) SetTimeout(duration time.Duration)                                      {}
func (nc *NoopCollector) CommitteeProcessingDuration(duration time.Duration)                     {}
func (nc *NoopCollector) SignerProcessingDuration(duration time.Duration)                        {}
func (nc *NoopCollector) ValidatorProcessingDuration(duration time.Duration)                     {}
func (nc *NoopCollector) PayloadProductionDuration(duration time.Duration)                       {}
func (nc *NoopCollector) TimeoutCollectorsRange(uint64, uint64, int)                             {}
func (nc *NoopCollector) TransactionIngested(txID flow.Identifier)                               {}
func (nc *NoopCollector) ClusterBlockProposed(*cluster.Block)                                    {}
func (nc *NoopCollector) ClusterBlockFinalized(*cluster.Block)                                   {}
func (nc *NoopCollector) StartCollectionToFinalized(collectionID flow.Identifier)                {}
func (nc *NoopCollector) FinishCollectionToFinalized(collectionID flow.Identifier)               {}
func (nc *NoopCollector) StartBlockToSeal(blockID flow.Identifier)                               {}
func (nc *NoopCollector) FinishBlockToSeal(blockID flow.Identifier)                              {}
func (nc *NoopCollector) EmergencySeal()                                                         {}
func (nc *NoopCollector) OnReceiptProcessingDuration(duration time.Duration)                     {}
func (nc *NoopCollector) OnApprovalProcessingDuration(duration time.Duration)                    {}
func (nc *NoopCollector) CheckSealingDuration(duration time.Duration)                            {}
func (nc *NoopCollector) OnExecutionResultReceivedAtAssignerEngine()                             {}
func (nc *NoopCollector) OnVerifiableChunkReceivedAtVerifierEngine()                             {}
func (nc *NoopCollector) OnResultApprovalDispatchedInNetworkByVerifier()                         {}
func (nc *NoopCollector) SetMaxChunkDataPackAttemptsForNextUnsealedHeightAtRequester(attempts uint64) {
}
func (nc *NoopCollector) OnFinalizedBlockArrivedAtAssigner(height uint64)                       {}
func (nc *NoopCollector) OnChunksAssignmentDoneAtAssigner(chunks int)                           {}
func (nc *NoopCollector) OnAssignedChunkProcessedAtAssigner()                                   {}
func (nc *NoopCollector) OnAssignedChunkReceivedAtFetcher()                                     {}
func (nc *NoopCollector) OnChunkDataPackRequestDispatchedInNetworkByRequester()                 {}
func (nc *NoopCollector) OnChunkDataPackRequestSentByFetcher()                                  {}
func (nc *NoopCollector) OnChunkDataPackRequestReceivedByRequester()                            {}
func (nc *NoopCollector) OnChunkDataPackArrivedAtFetcher()                                      {}
func (nc *NoopCollector) OnChunkDataPackSentToFetcher()                                         {}
func (nc *NoopCollector) OnVerifiableChunkSentToVerifier()                                      {}
func (nc *NoopCollector) OnBlockConsumerJobDone(uint64)                                         {}
func (nc *NoopCollector) OnChunkConsumerJobDone(uint64)                                         {}
func (nc *NoopCollector) OnChunkDataPackResponseReceivedFromNetworkByRequester()                {}
func (nc *NoopCollector) TotalConnectionsInPool(connectionCount uint, connectionPoolSize uint)  {}
func (nc *NoopCollector) ConnectionFromPoolReused()                                             {}
func (nc *NoopCollector) ConnectionAddedToPool()                                                {}
func (nc *NoopCollector) NewConnectionEstablished()                                             {}
func (nc *NoopCollector) ConnectionFromPoolInvalidated()                                        {}
func (nc *NoopCollector) ConnectionFromPoolUpdated()                                            {}
func (nc *NoopCollector) ConnectionFromPoolEvicted()                                            {}
func (nc *NoopCollector) StartBlockReceivedToExecuted(blockID flow.Identifier)                  {}
func (nc *NoopCollector) FinishBlockReceivedToExecuted(blockID flow.Identifier)                 {}
func (nc *NoopCollector) ExecutionComputationUsedPerBlock(computation uint64)                   {}
func (nc *NoopCollector) ExecutionStorageStateCommitment(bytes int64)                           {}
func (nc *NoopCollector) ExecutionLastExecutedBlockHeight(height uint64)                        {}
func (nc *NoopCollector) ExecutionLastFinalizedExecutedBlockHeight(height uint64)               {}
func (nc *NoopCollector) ExecutionBlockExecuted(_ time.Duration, _ module.ExecutionResultStats) {}
func (nc *NoopCollector) ExecutionCollectionExecuted(_ time.Duration, _ module.ExecutionResultStats) {
}
func (nc *NoopCollector) ExecutionBlockExecutionEffortVectorComponent(_ string, _ uint) {}
func (nc *NoopCollector) ExecutionBlockCachedPrograms(programs int)                     {}
func (nc *NoopCollector) ExecutionTransactionExecuted(_ time.Duration, _ int, _, _ uint64, _, _ int, _ bool) {
}
func (nc *NoopCollector) ExecutionChunkDataPackGenerated(_, _ int)                              {}
func (nc *NoopCollector) ExecutionScriptExecuted(dur time.Duration, compUsed, _, _ uint64)      {}
func (nc *NoopCollector) ForestApproxMemorySize(bytes uint64)                                   {}
func (nc *NoopCollector) ForestNumberOfTrees(number uint64)                                     {}
func (nc *NoopCollector) LatestTrieRegCount(number uint64)                                      {}
func (nc *NoopCollector) LatestTrieRegCountDiff(number int64)                                   {}
func (nc *NoopCollector) LatestTrieRegSize(size uint64)                                         {}
func (nc *NoopCollector) LatestTrieRegSizeDiff(size int64)                                      {}
func (nc *NoopCollector) LatestTrieMaxDepthTouched(maxDepth uint16)                             {}
func (nc *NoopCollector) UpdateCount()                                                          {}
func (nc *NoopCollector) ProofSize(bytes uint32)                                                {}
func (nc *NoopCollector) UpdateValuesNumber(number uint64)                                      {}
func (nc *NoopCollector) UpdateValuesSize(byte uint64)                                          {}
func (nc *NoopCollector) UpdateDuration(duration time.Duration)                                 {}
func (nc *NoopCollector) UpdateDurationPerItem(duration time.Duration)                          {}
func (nc *NoopCollector) ReadValuesNumber(number uint64)                                        {}
func (nc *NoopCollector) ReadValuesSize(byte uint64)                                            {}
func (nc *NoopCollector) ReadDuration(duration time.Duration)                                   {}
func (nc *NoopCollector) ReadDurationPerItem(duration time.Duration)                            {}
func (nc *NoopCollector) ExecutionCollectionRequestSent()                                       {}
func (nc *NoopCollector) ExecutionCollectionRequestRetried()                                    {}
func (nc *NoopCollector) RuntimeTransactionParsed(dur time.Duration)                            {}
func (nc *NoopCollector) RuntimeTransactionChecked(dur time.Duration)                           {}
func (nc *NoopCollector) RuntimeTransactionInterpreted(dur time.Duration)                       {}
func (nc *NoopCollector) RuntimeSetNumberOfAccounts(count uint64)                               {}
func (nc *NoopCollector) RuntimeTransactionProgramsCacheMiss()                                  {}
func (nc *NoopCollector) RuntimeTransactionProgramsCacheHit()                                   {}
func (nc *NoopCollector) ScriptExecuted(dur time.Duration, size int)                            {}
func (nc *NoopCollector) ScriptExecutionErrorLocal()                                            {}
func (nc *NoopCollector) ScriptExecutionErrorOnExecutionNode()                                  {}
func (nc *NoopCollector) ScriptExecutionResultMismatch()                                        {}
func (nc *NoopCollector) ScriptExecutionResultMatch()                                           {}
func (nc *NoopCollector) ScriptExecutionErrorMismatch()                                         {}
func (nc *NoopCollector) ScriptExecutionErrorMatch()                                            {}
func (nc *NoopCollector) ScriptExecutionNotIndexed()                                            {}
func (nc *NoopCollector) TransactionResultFetched(dur time.Duration, size int)                  {}
func (nc *NoopCollector) TransactionReceived(txID flow.Identifier, when time.Time)              {}
func (nc *NoopCollector) TransactionFinalized(txID flow.Identifier, when time.Time)             {}
func (nc *NoopCollector) TransactionExecuted(txID flow.Identifier, when time.Time)              {}
func (nc *NoopCollector) TransactionExpired(txID flow.Identifier)                               {}
func (nc *NoopCollector) TransactionSubmissionFailed()                                          {}
func (nc *NoopCollector) UpdateExecutionReceiptMaxHeight(height uint64)                         {}
func (nc *NoopCollector) UpdateLastFullBlockHeight(height uint64)                               {}
func (nc *NoopCollector) ChunkDataPackRequestProcessed()                                        {}
func (nc *NoopCollector) ExecutionSync(syncing bool)                                            {}
func (nc *NoopCollector) ExecutionBlockDataUploadStarted()                                      {}
func (nc *NoopCollector) ExecutionBlockDataUploadFinished(dur time.Duration)                    {}
func (nc *NoopCollector) ExecutionComputationResultUploaded()                                   {}
func (nc *NoopCollector) ExecutionComputationResultUploadRetried()                              {}
func (nc *NoopCollector) RootIDComputed(duration time.Duration, numberOfChunks int)             {}
func (nc *NoopCollector) AddBlobsSucceeded(duration time.Duration, totalSize uint64)            {}
func (nc *NoopCollector) AddBlobsFailed()                                                       {}
func (nc *NoopCollector) FulfilledHeight(blockHeight uint64)                                    {}
func (nc *NoopCollector) ReceiptSkipped()                                                       {}
func (nc *NoopCollector) RequestSucceeded(uint64, time.Duration, uint64, int)                   {}
func (nc *NoopCollector) RequestFailed(duration time.Duration, retryable bool)                  {}
func (nc *NoopCollector) RequestCanceled()                                                      {}
func (nc *NoopCollector) ResponseDropped()                                                      {}
func (nc *NoopCollector) Pruned(height uint64, duration time.Duration)                          {}
func (nc *NoopCollector) UpdateCollectionMaxHeight(height uint64)                               {}
func (nc *NoopCollector) BucketAvailableSlots(uint64, uint64)                                   {}
func (nc *NoopCollector) OnKeyPutSuccess(uint32)                                                {}
func (nc *NoopCollector) OnEntityEjectionDueToFullCapacity()                                    {}
func (nc *NoopCollector) OnEntityEjectionDueToEmergency()                                       {}
func (nc *NoopCollector) OnKeyGetSuccess()                                                      {}
func (nc *NoopCollector) OnKeyGetFailure()                                                      {}
func (nc *NoopCollector) OnKeyPutAttempt(uint32)                                                {}
func (nc *NoopCollector) OnKeyPutDrop()                                                         {}
func (nc *NoopCollector) OnKeyPutDeduplicated()                                                 {}
func (nc *NoopCollector) OnKeyRemoved(uint32)                                                   {}
func (nc *NoopCollector) ExecutionDataFetchStarted()                                            {}
func (nc *NoopCollector) ExecutionDataFetchFinished(_ time.Duration, _ bool, _ uint64)          {}
func (nc *NoopCollector) NotificationSent(height uint64)                                        {}
func (nc *NoopCollector) FetchRetried()                                                         {}
func (nc *NoopCollector) RoutingTablePeerAdded()                                                {}
func (nc *NoopCollector) RoutingTablePeerRemoved()                                              {}
func (nc *NoopCollector) PrunedBlockById(status *chainsync.Status)                              {}
func (nc *NoopCollector) PrunedBlockByHeight(status *chainsync.Status)                          {}
func (nc *NoopCollector) PrunedBlocks(totalByHeight, totalById, storedByHeight, storedById int) {}
func (nc *NoopCollector) RangeRequested(ran chainsync.Range)                                    {}
func (nc *NoopCollector) BatchRequested(batch chainsync.Batch)                                  {}
func (nc *NoopCollector) OnUnauthorizedMessage(role, msgType, topic, offense string)            {}
func (nc *NoopCollector) ObserveHTTPRequestDuration(context.Context, httpmetrics.HTTPReqProperties, time.Duration) {
}
func (nc *NoopCollector) ObserveHTTPResponseSize(context.Context, httpmetrics.HTTPReqProperties, int64) {
}
func (nc *NoopCollector) AddInflightRequests(context.Context, httpmetrics.HTTPProperties, int) {}
func (nc *NoopCollector) AddTotalRequests(context.Context, string, string)                     {}
func (nc *NoopCollector) OnRateLimitedPeer(pid peer.ID, role, msgType, topic, reason string) {
}
func (nc *NoopCollector) OnStreamCreated(duration time.Duration, attempts int)          {}
func (nc *NoopCollector) OnStreamCreationFailure(duration time.Duration, attempts int)  {}
func (nc *NoopCollector) OnPeerDialed(duration time.Duration, attempts int)             {}
func (nc *NoopCollector) OnPeerDialFailure(duration time.Duration, attempts int)        {}
func (nc *NoopCollector) OnStreamEstablished(duration time.Duration, attempts int)      {}
func (nc *NoopCollector) OnEstablishStreamFailure(duration time.Duration, attempts int) {}
func (nc *NoopCollector) OnDialRetryBudgetUpdated(budget uint64)                        {}
func (nc *NoopCollector) OnStreamCreationRetryBudgetUpdated(budget uint64)              {}
func (nc *NoopCollector) OnDialRetryBudgetResetToDefault()                              {}
func (nc *NoopCollector) OnStreamCreationRetryBudgetResetToDefault()                    {}

var _ module.HeroCacheMetrics = (*NoopCollector)(nil)

func (nc *NoopCollector) OnIWantControlMessageIdsTruncated(diff int)               {}
func (nc *NoopCollector) OnIWantMessageIDsReceived(msgIdCount int)                 {}
func (nc *NoopCollector) OnIHaveMessageIDsReceived(channel string, msgIdCount int) {}
func (nc *NoopCollector) OnLocalMeshSizeUpdated(string, int)                       {}
func (nc *NoopCollector) OnPeerAddedToProtocol(protocol string)                    {}
func (nc *NoopCollector) OnPeerRemovedFromProtocol()                               {}
func (nc *NoopCollector) OnLocalPeerJoinedTopic()                                  {}
func (nc *NoopCollector) OnLocalPeerLeftTopic()                                    {}
func (nc *NoopCollector) OnPeerGraftTopic(topic string)                            {}
func (nc *NoopCollector) OnPeerPruneTopic(topic string)                            {}
func (nc *NoopCollector) OnMessageEnteredValidation(size int)                      {}
func (nc *NoopCollector) OnMessageRejected(size int, reason string)                {}
func (nc *NoopCollector) OnMessageDuplicate(size int)                              {}
func (nc *NoopCollector) OnPeerThrottled()                                         {}
func (nc *NoopCollector) OnRpcReceived(msgCount int, iHaveCount int, iWantCount int, graftCount int, pruneCount int) {
}
func (nc *NoopCollector) OnRpcSent(msgCount int, iHaveCount int, iWantCount int, graftCount int, pruneCount int) {
}
func (nc *NoopCollector) OnOutboundRpcDropped()                                            {}
func (nc *NoopCollector) OnUndeliveredMessage()                                            {}
func (nc *NoopCollector) OnMessageDeliveredToAllSubscribers(size int)                      {}
func (nc *NoopCollector) AllowConn(network.Direction, bool)                                {}
func (nc *NoopCollector) BlockConn(network.Direction, bool)                                {}
func (nc *NoopCollector) AllowStream(peer.ID, network.Direction)                           {}
func (nc *NoopCollector) BlockStream(peer.ID, network.Direction)                           {}
func (nc *NoopCollector) AllowPeer(peer.ID)                                                {}
func (nc *NoopCollector) BlockPeer(peer.ID)                                                {}
func (nc *NoopCollector) AllowProtocol(protocol.ID)                                        {}
func (nc *NoopCollector) BlockProtocol(protocol.ID)                                        {}
func (nc *NoopCollector) BlockProtocolPeer(protocol.ID, peer.ID)                           {}
func (nc *NoopCollector) AllowService(string)                                              {}
func (nc *NoopCollector) BlockService(string)                                              {}
func (nc *NoopCollector) BlockServicePeer(string, peer.ID)                                 {}
func (nc *NoopCollector) AllowMemory(int)                                                  {}
func (nc *NoopCollector) BlockMemory(int)                                                  {}
func (nc *NoopCollector) SetWarningStateCount(u uint)                                      {}
func (nc *NoopCollector) OnInvalidMessageDeliveredUpdated(topic channels.Topic, f float64) {}
func (nc *NoopCollector) OnMeshMessageDeliveredUpdated(topic channels.Topic, f float64)    {}
func (nc *NoopCollector) OnFirstMessageDeliveredUpdated(topic channels.Topic, f float64)   {}
func (nc *NoopCollector) OnTimeInMeshUpdated(topic channels.Topic, duration time.Duration) {}
func (nc *NoopCollector) OnBehaviourPenaltyUpdated(f float64)                              {}
func (nc *NoopCollector) OnIPColocationFactorUpdated(f float64)                            {}
func (nc *NoopCollector) OnAppSpecificScoreUpdated(f float64)                              {}
func (nc *NoopCollector) OnOverallPeerScoreUpdated(f float64)                              {}
func (nc *NoopCollector) OnIHaveControlMessageIdsTruncated(diff int)                       {}
func (nc *NoopCollector) OnControlMessagesTruncated(messageType p2pmsg.ControlMessageType, diff int) {
}
func (nc *NoopCollector) OnIncomingRpcReceived(iHaveCount, iWantCount, graftCount, pruneCount, msgCount int) {
}
func (nc *NoopCollector) AsyncProcessingStarted()                                                 {}
func (nc *NoopCollector) AsyncProcessingFinished(time.Duration)                                   {}
func (nc *NoopCollector) OnIWantMessagesInspected(duplicateCount int, cacheMissCount int)         {}
func (nc *NoopCollector) OnIWantDuplicateMessageIdsExceedThreshold()                              {}
func (nc *NoopCollector) OnIWantCacheMissMessageIdsExceedThreshold()                              {}
func (nc *NoopCollector) OnIHaveMessagesInspected(duplicateTopicIds int, duplicateMessageIds int) {}
func (nc *NoopCollector) OnIHaveDuplicateTopicIdsExceedThreshold()                                {}
func (nc *NoopCollector) OnIHaveDuplicateMessageIdsExceedThreshold()                              {}
func (nc *NoopCollector) OnInvalidTopicIdDetectedForControlMessage(messageType p2pmsg.ControlMessageType) {
}
func (nc *NoopCollector) OnActiveClusterIDsNotSetErr()                      {}
func (nc *NoopCollector) OnUnstakedPeerInspectionFailed()                   {}
func (nc *NoopCollector) OnInvalidControlMessageNotificationSent()          {}
func (nc *NoopCollector) OnPublishMessagesInspectionErrorExceedsThreshold() {}
func (nc *NoopCollector) OnPruneDuplicateTopicIdsExceedThreshold()          {}
func (nc *NoopCollector) OnPruneMessageInspected(duplicateTopicIds int)     {}
func (nc *NoopCollector) OnGraftDuplicateTopicIdsExceedThreshold()          {}
func (nc *NoopCollector) OnGraftMessageInspected(duplicateTopicIds int)     {}
func (nc *NoopCollector) OnPublishMessageInspected(totalErrCount int, invalidTopicIdsCount int, invalidSubscriptionsCount int, invalidSendersCount int) {
}

func (nc *NoopCollector) OnMisbehaviorReported(string, string) {}
func (nc *NoopCollector) OnViolationReportSkipped()            {}

var _ ObserverMetrics = (*NoopCollector)(nil)

func (nc *NoopCollector) RecordRPC(handler, rpc string, code codes.Code) {}

var _ module.ExecutionStateIndexerMetrics = (*NoopCollector)(nil)

func (nc *NoopCollector) BlockIndexed(uint64, time.Duration, int, int, int) {}
func (nc *NoopCollector) BlockReindexed()                                   {}
func (nc *NoopCollector) InitializeLatestHeight(height uint64)              {}
