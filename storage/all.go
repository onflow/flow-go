package storage

// All includes all the storage modules
type All struct {
	Headers                        Headers
	Guarantees                     Guarantees
	Seals                          Seals
	Index                          Index
	Payloads                       Payloads
	Blocks                         Blocks
	QuorumCertificates             QuorumCertificates
	Setups                         EpochSetups
	EpochCommits                   EpochCommits
	Results                        ExecutionResults
	Receipts                       ExecutionReceipts
	ChunkDataPacks                 ChunkDataPacks
	Commits                        Commits
	Transactions                   Transactions
	LightTransactionResults        LightTransactionResults
	TransactionResults             TransactionResults
	TransactionResultErrorMessages TransactionResultErrorMessages
	Collections                    Collections
	Events                         Events
	EpochProtocolStateEntries      EpochProtocolStateEntries
	ProtocolKVStore                ProtocolKVStore
	VersionBeacons                 VersionBeacons
	RegisterIndex                  RegisterIndex
}
