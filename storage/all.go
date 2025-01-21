package storage

// All includes all the storage modules
type All struct {
	Headers                   Headers
	Guarantees                Guarantees
	Seals                     Seals
	Index                     Index
	Payloads                  Payloads
	Blocks                    Blocks
	QuorumCertificates        QuorumCertificates
	Setups                    EpochSetups
	EpochCommits              EpochCommits
	ChunkDataPacks            ChunkDataPacks
	Transactions              Transactions
	Collections               Collections
	EpochProtocolStateEntries EpochProtocolStateEntries
	ProtocolKVStore           ProtocolKVStore
	VersionBeacons            VersionBeacons
	RegisterIndex             RegisterIndex

	Results  ExecutionResults
	Receipts ExecutionReceipts
}

type Execution struct {
	Results            ExecutionResults
	Receipts           ExecutionReceipts
	Commits            Commits
	TransactionResults TransactionResults
	Events             Events
}
