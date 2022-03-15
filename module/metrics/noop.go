package metrics

import (
	"time"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

type NoopCollector struct{}

func NewNoopCollector() *NoopCollector {
	nc := &NoopCollector{}
	return nc
}

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
func (nc *NoopCollector) BlockProposed(*flow.Block)                                              {}
func (nc *NoopCollector) BlockFinalized(*flow.Block)                                             {}
func (nc *NoopCollector) BlockSealed(*flow.Block)                                                {}
func (nc *NoopCollector) BlockProposalDuration(duration time.Duration)                           {}
func (nc *NoopCollector) CommittedEpochFinalView(view uint64)                                    {}
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
func (nc *NoopCollector) CountSkipped()                                                          {}
func (nc *NoopCollector) CountTimeout()                                                          {}
func (nc *NoopCollector) SetTimeout(duration time.Duration)                                      {}
func (nc *NoopCollector) CommitteeProcessingDuration(duration time.Duration)                     {}
func (nc *NoopCollector) SignerProcessingDuration(duration time.Duration)                        {}
func (nc *NoopCollector) ValidatorProcessingDuration(duration time.Duration)                     {}
func (nc *NoopCollector) PayloadProductionDuration(duration time.Duration)                       {}
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
func (nc *NoopCollector) StartBlockReceivedToExecuted(blockID flow.Identifier)                  {}
func (nc *NoopCollector) FinishBlockReceivedToExecuted(blockID flow.Identifier)                 {}
func (nc *NoopCollector) ExecutionComputationUsedPerBlock(computation uint64)                   {}
func (nc *NoopCollector) ExecutionStateReadsPerBlock(reads uint64)                              {}
func (nc *NoopCollector) ExecutionStorageStateCommitment(bytes int64)                           {}
func (nc *NoopCollector) ExecutionLastExecutedBlockHeight(height uint64)                        {}
func (nc *NoopCollector) ExecutionBlockExecuted(_ time.Duration, _ uint64, _ int, _ int)        {}
func (nc *NoopCollector) ExecutionCollectionExecuted(_ time.Duration, _ uint64, _ int)          {}
func (nc *NoopCollector) ExecutionTransactionExecuted(_ time.Duration, _ uint64, _ int, _ bool) {}
func (nc *NoopCollector) ExecutionScriptExecuted(dur time.Duration, compUsed uint64)            {}
func (nc *NoopCollector) ForestApproxMemorySize(bytes uint64)                                   {}
func (nc *NoopCollector) ForestNumberOfTrees(number uint64)                                     {}
func (nc *NoopCollector) LatestTrieRegCount(number uint64)                                      {}
func (nc *NoopCollector) LatestTrieRegCountDiff(number int64)                                   {}
func (nc *NoopCollector) LatestTrieMaxDepth(number uint64)                                      {}
func (nc *NoopCollector) LatestTrieMaxDepthDiff(number int64)                                   {}
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
func (nc *NoopCollector) TransactionReceived(txID flow.Identifier, when time.Time)              {}
func (nc *NoopCollector) TransactionFinalized(txID flow.Identifier, when time.Time)             {}
func (nc *NoopCollector) TransactionExecuted(txID flow.Identifier, when time.Time)              {}
func (nc *NoopCollector) TransactionExpired(txID flow.Identifier)                               {}
func (nc *NoopCollector) TransactionSubmissionFailed()                                          {}
func (nc *NoopCollector) ChunkDataPackRequested()                                               {}
func (nc *NoopCollector) ExecutionSync(syncing bool)                                            {}
func (nc *NoopCollector) DiskSize(uint64)                                                       {}
func (nc *NoopCollector) ExecutionBlockDataUploadStarted()                                      {}
func (nc *NoopCollector) ExecutionBlockDataUploadFinished(dur time.Duration)                    {}
func (nc *NoopCollector) ExecutionDataAddStarted()                                              {}
func (nc *NoopCollector) ExecutionDataAddFinished(time.Duration, bool, uint64)                  {}
func (nc *NoopCollector) ExecutionDataGetStarted()                                              {}
func (nc *NoopCollector) ExecutionDataGetFinished(time.Duration, bool, uint64)                  {}
