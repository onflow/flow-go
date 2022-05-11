package meter

import "github.com/onflow/cadence/runtime/common"

type MetringOperationType uint

const (
	// [2_000, 3_000) reserved for the FVM
	_ common.ComputationKind = iota + 2_000
	ComputationKindHash
	ComputationKindVerifySignature
	ComputationKindAddAccountKey
	ComputationKindAddEncodedAccountKey
	ComputationKindAllocateStorageIndex
	ComputationKindCreateAccount
	ComputationKindEmitEvent
	ComputationKindGenerateUUID
	ComputationKindGetAccountAvailableBalance
	ComputationKindGetAccountBalance
	ComputationKindGetAccountContractCode
	ComputationKindGetAccountContractNames
	ComputationKindGetAccountKey
	ComputationKindGetBlockAtHeight
	ComputationKindGetCode
	ComputationKindGetCurrentBlockHeight
	ComputationKindGetProgram
	ComputationKindGetStorageCapacity
	ComputationKindGetStorageUsed
	ComputationKindGetValue
	ComputationKindRemoveAccountContractCode
	ComputationKindResolveLocation
	ComputationKindRevokeAccountKey
	ComputationKindRevokeEncodedAccountKey
	ComputationKindSetProgram
	ComputationKindSetValue
	ComputationKindUpdateAccountContractCode
	ComputationKindValidatePublicKey
	ComputationKindValueExists
)

type MeteredIntensities map[common.ComputationKind]uint

type Meter interface {
	// merge child funcionality
	NewChild() Meter
	MergeMeter(child Meter, enforceLimits bool) error

	// computation metering
	MeterComputation(kind common.ComputationKind, intensity uint) error
	ComputationIntensities() MeteredIntensities
	TotalComputationUsed() uint
	TotalComputationLimit() uint

	// memory metering
	MeterMemory(kind common.ComputationKind, intensity uint) error
	MemoryIntensities() MeteredIntensities
	TotalMemoryUsed() uint
	TotalMemoryLimit() uint

	// TODO move storage metering to here
	// MeterStorageRead(byteSize uint) error
	// MeterStorageWrite(byteSize uint) error
	// TotalBytesReadFromStorage() int
	// TotalBytesWroteToStorage() int
	// TotalBytesOfStorageInteractions() int
}
