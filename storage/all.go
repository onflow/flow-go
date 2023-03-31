package storage

// All includes all the storage modules
type All[wb WriteBatch, tx Transaction] struct {
	Headers            Headers[wb]
	Guarantees         Guarantees
	Seals              Seals
	Index              Index
	Payloads           Payloads
	Blocks             Blocks[tx]
	QuorumCertificates QuorumCertificates[tx]
	Setups             EpochSetups[tx]
	EpochCommits       EpochCommits[tx]
	Statuses           EpochStatuses[tx]
	Results            ExecutionResults[wb, tx]
	Receipts           ExecutionReceipts[wb]
	ChunkDataPacks     ChunkDataPacks[wb]
	Commits            Commits[wb]
	Transactions       Transactions
	TransactionResults TransactionResults[wb]
	Collections        Collections
	Events             Events[wb]
}
