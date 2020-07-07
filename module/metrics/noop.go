package metrics

import (
	"time"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
)

type NoopCollector struct{}

func NewNoopCollector() *NoopCollector {
	nc := &NoopCollector{}
	return nc
}

func (nc *NoopCollector) NetworkMessageSent(sizeBytes int, topic string)            {}
func (nc *NoopCollector) NetworkMessageReceived(sizeBytes int, topic string)        {}
func (nc *NoopCollector) NetworkDuplicateMessagesDropped(topic string)              {}
func (nc *NoopCollector) MessageSent(engine string, message string)                 {}
func (nc *NoopCollector) MessageReceived(engine string, message string)             {}
func (nc *NoopCollector) MessageHandled(engine string, message string)              {}
func (nc *NoopCollector) RanGC(duration time.Duration)                              {}
func (nc *NoopCollector) BadgerLSMSize(sizeBytes int64)                             {}
func (nc *NoopCollector) BadgerVLogSize(sizeBytes int64)                            {}
func (nc *NoopCollector) BadgerNumReads(n int64)                                    {}
func (nc *NoopCollector) BadgerNumWrites(n int64)                                   {}
func (nc *NoopCollector) BadgerNumBytesRead(n int64)                                {}
func (nc *NoopCollector) BadgerNumBytesWritten(n int64)                             {}
func (nc *NoopCollector) BadgerNumGets(n int64)                                     {}
func (nc *NoopCollector) BadgerNumPuts(n int64)                                     {}
func (nc *NoopCollector) BadgerNumBlockedPuts(n int64)                              {}
func (nc *NoopCollector) BadgerNumMemtableGets(n int64)                             {}
func (nc *NoopCollector) FinalizedHeight(height uint64)                             {}
func (nc *NoopCollector) SealedHeight(height uint64)                                {}
func (nc *NoopCollector) BlockProposed(*flow.Block)                                 {}
func (nc *NoopCollector) BlockFinalized(*flow.Block)                                {}
func (nc *NoopCollector) BlockSealed(*flow.Block)                                   {}
func (nc *NoopCollector) CacheEntries(resource string, entries uint)                {}
func (nc *NoopCollector) CacheHit(resource string)                                  {}
func (nc *NoopCollector) CacheMiss(resource string)                                 {}
func (nc *NoopCollector) MempoolEntries(resource string, entries uint)              {}
func (nm *NoopCollector) Register(resource string, entriesFunc EntriesFunc) error   { return nil }
func (nc *NoopCollector) HotStuffBusyDuration(duration time.Duration, event string) {}
func (nc *NoopCollector) HotStuffIdleDuration(duration time.Duration)               {}
func (nc *NoopCollector) HotStuffWaitDuration(duration time.Duration, event string) {}
func (nc *NoopCollector) SetCurView(view uint64)                                    {}
func (nc *NoopCollector) SetQCView(view uint64)                                     {}
func (nc *NoopCollector) CountSkipped()                                             {}
func (nc *NoopCollector) CountTimeout()                                             {}
func (nc *NoopCollector) SetTimeout(duration time.Duration)                         {}
func (nc *NoopCollector) CommitteeProcessingDuration(duration time.Duration)        {}
func (nc *NoopCollector) SignerProcessingDuration(duration time.Duration)           {}
func (nc *NoopCollector) ValidatorProcessingDuration(duration time.Duration)        {}
func (nc *NoopCollector) PayloadProductionDuration(duration time.Duration)          {}
func (nc *NoopCollector) TransactionIngested(txID flow.Identifier)                  {}
func (nc *NoopCollector) ClusterBlockProposed(*cluster.Block)                       {}
func (nc *NoopCollector) ClusterBlockFinalized(*cluster.Block)                      {}
func (nc *NoopCollector) StartCollectionToFinalized(collectionID flow.Identifier)   {}
func (nc *NoopCollector) FinishCollectionToFinalized(collectionID flow.Identifier)  {}
func (nc *NoopCollector) StartBlockToSeal(blockID flow.Identifier)                  {}
func (nc *NoopCollector) FinishBlockToSeal(blockID flow.Identifier)                 {}
func (nc *NoopCollector) OnExecutionReceiptReceived()                               {}
func (nc *NoopCollector) OnExecutionResultSent()                                    {}
func (nc *NoopCollector) OnExecutionResultReceived()                                {}
func (nc *NoopCollector) OnVerifiableChunkSent()                                    {}
func (nc *NoopCollector) OnVerifiableChunkReceived()                                {}
func (nc *NoopCollector) OnChunkDataPackReceived()                                  {}
func (nc *NoopCollector) OnResultApproval()                                         {}
func (nc *NoopCollector) OnChunkVerificationStarted(chunkID flow.Identifier)        {}
func (nc *NoopCollector) OnChunkVerificationFinished(chunkID flow.Identifier)       {}
func (nc *NoopCollector) LogVerifiableChunkSize(size float64)                       {}
func (nc *NoopCollector) StartBlockReceivedToExecuted(blockID flow.Identifier)      {}
func (nc *NoopCollector) FinishBlockReceivedToExecuted(blockID flow.Identifier)     {}
func (nc *NoopCollector) ExecutionGasUsedPerBlock(gas uint64)                       {}
func (nc *NoopCollector) ExecutionStateReadsPerBlock(reads uint64)                  {}
func (nc *NoopCollector) ExecutionStateStorageDiskTotal(bytes int64)                {}
func (nc *NoopCollector) ExecutionStorageStateCommitment(bytes int64)               {}
func (nc *NoopCollector) ExecutionLastExecutedBlockView(view uint64)                {}
func (ec *NoopCollector) ExecutionTotalExecutedTransactions(numberOfTx int)         {}
func (nc *NoopCollector) ForestApproxMemorySize(bytes uint64)                       {}
func (nc *NoopCollector) ForestNumberOfTrees(number uint64)                         {}
func (nc *NoopCollector) LatestTrieRegCount(number uint64)                          {}
func (nc *NoopCollector) LatestTrieRegCountDiff(number uint64)                      {}
func (nc *NoopCollector) LatestTrieMaxDepth(number uint64)                          {}
func (nc *NoopCollector) LatestTrieMaxDepthDiff(number uint64)                      {}
func (nc *NoopCollector) UpdateCount()                                              {}
func (nc *NoopCollector) ProofSize(bytes uint32)                                    {}
func (nc *NoopCollector) UpdateValuesNumber(number uint64)                          {}
func (nc *NoopCollector) UpdateValuesSize(byte uint64)                              {}
func (nc *NoopCollector) UpdateDuration(duration time.Duration)                     {}
func (nc *NoopCollector) UpdateDurationPerItem(duration time.Duration)              {}
func (nc *NoopCollector) ReadValuesNumber(number uint64)                            {}
func (nc *NoopCollector) ReadValuesSize(byte uint64)                                {}
func (nc *NoopCollector) ReadDuration(duration time.Duration)                       {}
func (nc *NoopCollector) ReadDurationPerItem(duration time.Duration)                {}
func (nc *NoopCollector) ExecutionCollectionRequestSent()                           {}
func (nc *NoopCollector) ExecutionCollectionRequestRetried()                        {}
func (nc *NoopCollector) TransactionParsed(dur time.Duration)                       {}
func (nc *NoopCollector) TransactionChecked(dur time.Duration)                      {}
func (nc *NoopCollector) TransactionInterpreted(dur time.Duration)                  {}
func (nc *NoopCollector) TransactionReceived(txID flow.Identifier, when time.Time)  {}
func (nc *NoopCollector) TransactionFinalized(txID flow.Identifier, when time.Time) {}
func (nc *NoopCollector) TransactionExecuted(txID flow.Identifier, when time.Time)  {}
