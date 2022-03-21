package meter

import "github.com/onflow/cadence/runtime/common"

type MetringOperationType uint

// continuation of cadence.ComputationKind
const (
	// [2_000, 3_000) reserved for the FVM
	_ = iota + 2_000
	ComputationKindContractFunctionInvoke
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

// MeterInternalPrecisionBytes are the amount of bytes that are used internally by the Meter
// to allow for metering computation smaller than one unit of computation. This allows for more fine weights.
// A weight of 1 unit of computation is equal to 1<<16.
const MeterInternalPrecisionBytes = 16

var (
	// DefaultComputationWeights is the default weights for computation intensities
	// these weighs make the computation metering the same as it was before dynamic execution fees
	DefaultComputationWeights = map[uint]uint64{
		uint(common.ComputationKindStatement):          1 << MeterInternalPrecisionBytes,
		uint(common.ComputationKindLoop):               1 << MeterInternalPrecisionBytes,
		uint(common.ComputationKindFunctionInvocation): 1 << MeterInternalPrecisionBytes,
	}
	// DefaultMemoryWeights are empty as memory metering is not fully defined yet
	DefaultMemoryWeights = map[uint]uint64{}
)

type Meter interface {
	// merge child funcionality
	NewChild() Meter
	MergeMeter(child Meter) error

	// computation metering
	SetComputationWeights(map[uint]uint64)
	MeterComputation(kind uint, intensity uint) error
	ComputationIntensities() map[uint]uint
	TotalComputationUsed() uint
	TotalComputationLimit() uint

	// memory metering
	SetMemoryWeights(map[uint]uint64)
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
