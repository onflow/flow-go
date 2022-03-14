package meter

type MetringOperationType uint

// continuation of cadence.ComputationKind
const (
	// [2_000, 3_000) reserved for the FVM
	_ = iota + 2_000
	MeteredOperationContractFunctionInvoke
	MeteredOperationHash
	MeteredOperationVerifySignature
	MeteredOperationAddAccountKey
	MeteredOperationAddEncodedAccountKey
	MeteredOperationAllocateStorageIndex
	MeteredOperationCreateAccount
	MeteredOperationEmitEvent
	MeteredOperationGenerateUUID
	MeteredOperationGetAccountAvailableBalance
	MeteredOperationGetAccountBalance
	MeteredOperationGetAccountContractCode
	MeteredOperationGetAccountContractNames
	MeteredOperationGetAccountKey
	MeteredOperationGetBlockAtHeight
	MeteredOperationGetCode
	MeteredOperationGetCurrentBlockHeight
	MeteredOperationGetProgram
	MeteredOperationGetStorageCapacity
	MeteredOperationGetStorageUsed
	MeteredOperationGetValue
	MeteredOperationRemoveAccountContractCode
	MeteredOperationResolveLocation
	MeteredOperationRevokeAccountKey
	MeteredOperationRevokeEncodedAccountKey
	MeteredOperationSetProgram
	MeteredOperationSetValue
	MeteredOperationUpdateAccountContractCode
	MeteredOperationValidatePublicKey
	MeteredOperationValueExists
)

type Meter interface {
	// merge child funcionality
	NewChild() Meter
	MergeMeter(child Meter) error

	// computation metering
	MeterComputation(kind uint, intensity uint) error
	ComputationIntensities() map[uint]uint
	TotalComputationUsed() uint
	TotalComputationLimit() uint

	// memory metering
	MeterMemory(kind uint, intensity uint) error
	MemoryIntensities() map[uint]uint
	TotalMemoryUsed() uint
	TotalMemoryLimit() uint

	// TODO move storage metering to here
	// MeterStorageRead(byteSize uint) error
	// MeterStorageWrite(byteSize uint) error
	// TotalBytesReadFromStorage() int
	// TotalBytesWroteToStorage() int
	// TotalBytesOfStorageInteractions() int
}
