package metrics

import (
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
)

type NoopCollector struct {
}

func NewNoopCollector() *NoopCollector {
	nc := &NoopCollector{}
	return nc
}

func (nc *NoopCollector) NetworkMessageSent(sizeBytes int, topic string)            {}
func (nc *NoopCollector) NetworkMessageReceived(sizeBytes int, topic string)        {}
func (nc *NoopCollector) MessageSent(engine string, message string)                 {}
func (nc *NoopCollector) MessageReceived(engine string, message string)             {}
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
func (nc *NoopCollector) HotStuffBusyDuration(duration time.Duration, event string) {}
func (nc *NoopCollector) HotStuffIdleDuration(duration time.Duration)               {}
func (nc *NoopCollector) HotStuffWaitDuration(duration time.Duration, event string) {}
func (nc *NoopCollector) SetCurView(view uint64)                                    {}
func (nc *NoopCollector) SetQCView(view uint64)                                     {}
func (nc *NoopCollector) CountSkipped()                                             {}
func (nc *NoopCollector) CountTimeout()                                             {}
func (nc *NoopCollector) SetTimeout(duration time.Duration)                         {}
func (nc *NoopCollector) TransactionReceived(txID flow.Identifier)                  {}
func (nc *NoopCollector) CollectionProposed(collection flow.LightCollection)        {}
func (nc *NoopCollector) CollectionGuaranteed(collection flow.LightCollection)      {}
func (nc *NoopCollector) PendingClusterBlocks(n uint)                               {}
func (nc *NoopCollector) StartCollectionToFinalized(collectionID flow.Identifier)   {}
func (nc *NoopCollector) FinishCollectionToFinalized(collectionID flow.Identifier)  {}
func (nc *NoopCollector) StartBlockToSeal(blockID flow.Identifier)                  {}
func (nc *NoopCollector) FinishBlockToSeal(blockID flow.Identifier)                 {}
func (nc *NoopCollector) OnChunkVerificationStarted(chunkID flow.Identifier)        {}
func (nc *NoopCollector) OnChunkVerificationFinished(chunkID flow.Identifier)       {}
func (nc *NoopCollector) OnResultApproval()                                         {}
func (nc *NoopCollector) OnChunkDataAdded(chunkID flow.Identifier, size float64)    {}
func (nc *NoopCollector) OnChunkDataRemoved(chunkID flow.Identifier, size float64)  {}
func (nc *NoopCollector) StartBlockReceivedToExecuted(blockID flow.Identifier)      {}
func (nc *NoopCollector) FinishBlockReceivedToExecuted(blockID flow.Identifier)     {}
func (nc *NoopCollector) ExecutionGasUsedPerBlock(gas uint64)                       {}
func (nc *NoopCollector) ExecutionStateReadsPerBlock(reads uint64)                  {}
func (nc *NoopCollector) ExecutionStateStorageDiskTotal(bytes int64)                {}
func (nc *NoopCollector) ExecutionStorageStateCommitment(bytes int64)               {}
func (nc *NoopCollector) ExecutionLastExecutedBlockView(view uint64)                {}
func (ec *NoopCollector) ExecutionTotalExecutedTransactions(numberOfTx int)         {}
