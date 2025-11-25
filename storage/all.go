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

	// These results are for reading and storing the result data from block payload
	// EN uses a different results module to store their own results
	// and receipts (see the Execution struct below)
	Results  ExecutionResults
	Receipts ExecutionReceipts
}
