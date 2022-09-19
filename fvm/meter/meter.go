package meter

import (
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/model/flow"
)

type MetringOperationType uint

// TODO(patrick): rm after emulator is updated ...
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
type MeteredStorageInteractionMap map[StorageInteractionKey]uint64

type Meter interface {
	// merge child funcionality
	NewChild() Meter
	MergeMeter(child Meter)

	// computation metering
	MeterComputation(kind common.ComputationKind, intensity uint) error
	ComputationIntensities() MeteredComputationIntensities
	TotalComputationUsed() uint
	TotalComputationLimit() uint

	// memory metering
	MeterMemory(kind common.MemoryKind, intensity uint) error
	MemoryIntensities() MeteredMemoryIntensities
	TotalMemoryEstimate() uint64
	TotalMemoryLimit() uint64

	// storage metering
	MeterStorageRead(storageKey StorageInteractionKey, value flow.RegisterValue, enforceLimit bool) error
	MeterStorageWrite(storageKey StorageInteractionKey, value flow.RegisterValue, enforceLimit bool) error
	StorageUpdateSizeMap() MeteredStorageInteractionMap
	TotalBytesReadFromStorage() uint64
	TotalBytesWrittenToStorage() uint64
	TotalBytesOfStorageInteractions() uint64
}

type StorageInteractionKey struct {
	Owner, Key string
}
