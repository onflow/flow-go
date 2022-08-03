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
	// NewChild construct a new Meter instance with the same limits as parent
	NewChild() Meter

	// MergeMeter merges the input meter into the current meter and checks for
	// the limits.
	MergeMeter(child Meter, enforceLimits bool) error

	// Observer keeps track of all usage
	Observer() *WeightedMeter

	// Enforcer only keeps track of enforceable usage
	Enforcer() *WeightedMeter

	//
	// Computation metering
	//

	// MeterComputation captures computation usage and returns an error if it
	// goes beyond the limit.
	MeterComputation(kind common.ComputationKind, intensity uint) error

	// EnforcedComputationIntensities returns all the measured computational
	// intensities, as measured by the observer meter.
	ObservedComputationIntensities() MeteredComputationIntensities

	// EnforcedComputationIntensities returns all the measured computational
	// intensities, as measured by the enforcer meter.
	EnforcedComputationIntensities() MeteredComputationIntensities

	// TotalEnforcedComputationUsed returns the total computation used, as
	// measured by the observer meter.
	TotalObservedComputationUsed() uint

	// TotalEnforcedComputationUsed returns the total computation used, as
	// measured by the enforcer meter.
	TotalEnforcedComputationUsed() uint

	// TotalEnforcedComputationLimit returns the total computation limit.
	TotalEnforcedComputationLimit() uint

	//
	// Memory metering
	//

	// MeterMemory captures memory usage and returns an error if it goes
	// beyond the limit.
	MeterMemory(kind common.MemoryKind, intensity uint) error

	// ObservedMemoryIntensities returns all the measured memory intensities,
	// as measured by the observer meter.
	ObservedMemoryIntensities() MeteredMemoryIntensities

	// EnforcedMemoryIntensities returns all the measured memory intensities,
	// as measured by the enforcer meter.
	EnforcedMemoryIntensities() MeteredMemoryIntensities

	// TotalObservedMemoryEstimate returns the total memory used, as measured
	// by the observer meter.
	TotalObservedMemoryEstimate() uint

	// TotalEnforcedMemoryEstimate returns the total memory used, as measured
	// by the enforcer meter.
	TotalEnforcedMemoryEstimate() uint

	// TotalEnforcedMemoryLimit returns the total memory limit.
	TotalEnforcedMemoryLimit() uint

	// TODO(patrick): make these non-optional arguments to NewMeter
	SetComputationWeights(weights ExecutionEffortWeights)
	SetMemoryWeights(weights ExecutionMemoryWeights)
	SetTotalMemoryLimit(limit uint64)

	// TODO move storage metering to here
	// MeterStorageRead(byteSize uint) error
	// MeterStorageWrite(byteSize uint) error
	// TotalBytesReadFromStorage() int
	// TotalBytesWroteToStorage() int
	// TotalBytesOfStorageInteractions() int
}
