package storage

// All includes all the storage modules
type All struct {
	Headers            Headers
	Guarantees         Guarantees
	Seals              Seals
	Index              Index
	Payloads           Payloads
	Blocks             Blocks
	Setups             EpochSetups
	EpochCommits       EpochCommits
	Statuses           EpochStatuses
	Results            ExecutionResults
	Receipts           ExecutionReceipts
	ChunkDataPacks     ChunkDataPacks
	Commits            Commits
	Transactions       Transactions
	TransactionResults TransactionResults
	Collections        Collections
	Events             Events
}
