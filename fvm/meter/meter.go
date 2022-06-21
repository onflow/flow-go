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

type MeteredComputationIntensities map[common.ComputationKind]uint
type MeteredMemoryIntensities map[common.MemoryKind]uint

type Meter interface {
	// merge child funcionality
	NewChild() Meter
	MergeMeter(child Meter) error

	// computation metering
	MeterComputation(kind common.ComputationKind, intensity uint) error
	ComputationIntensities() MeteredComputationIntensities
	TotalComputationUsed() uint

	// memory metering
	MeterMemory(kind common.MemoryKind, intensity uint) error
	MemoryIntensities() MeteredMemoryIntensities
	TotalMemoryUsed() uint

	// interaction metering
	MeterRead(size uint64) error
	MeterWrite(previousSize uint64, newSize uint64) error
	MeterNewWrite(size uint64) error
	ReadCounter() uint64
	WriteCounter() uint64
	TotalBytesRead() uint64
	TotalBytesWritten() uint64
	TotalInteractionUsed() uint64

	// TODO move storage metering to here
	// MeterStorageRead(byteSize uint) error
	// MeterStorageWrite(byteSize uint) error
	// TotalBytesReadFromStorage() int
	// TotalBytesWroteToStorage() int
	// TotalBytesOfStorageInteractions() int
}
