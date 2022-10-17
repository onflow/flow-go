package meter

import (
	"math"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

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

type StorageInteractionKey struct {
	Owner, Key string
}

// MeterExecutionInternalPrecisionBytes are the amount of bytes that are used internally by the WeigthedMeter
// to allow for metering computation smaller than one unit of computation. This allows for more fine weights.
// A weight of 1 unit of computation is equal to 1<<16. The minimum possible weight is 1/65536.
const MeterExecutionInternalPrecisionBytes = 16

type ExecutionEffortWeights map[common.ComputationKind]uint64
type ExecutionMemoryWeights map[common.MemoryKind]uint64

var (
	// DefaultComputationWeights is the default weights for computation intensities
	// these weighs make the computation metering the same as it was before dynamic execution fees
	DefaultComputationWeights = ExecutionEffortWeights{
		common.ComputationKindStatement:          1 << MeterExecutionInternalPrecisionBytes,
		common.ComputationKindLoop:               1 << MeterExecutionInternalPrecisionBytes,
		common.ComputationKindFunctionInvocation: 1 << MeterExecutionInternalPrecisionBytes,
	}

	// DefaultMemoryWeights are currently hard-coded here. In the future we might like to
	// define this in a contract similar to the computation weights
	DefaultMemoryWeights = ExecutionMemoryWeights{

		// Values

		common.MemoryKindBoolValue:      8,
		common.MemoryKindAddressValue:   32,
		common.MemoryKindStringValue:    138,
		common.MemoryKindCharacterValue: 24,
		common.MemoryKindNumberValue:    8,
		// weights for these values include the cost of the Go struct itself (first number)
		// as well as the overhead for creation of the underlying atree (second number)
		common.MemoryKindArrayValueBase:           57 + 48,
		common.MemoryKindDictionaryValueBase:      33 + 96,
		common.MemoryKindCompositeValueBase:       233 + 96,
		common.MemoryKindSimpleCompositeValue:     73,
		common.MemoryKindSimpleCompositeValueBase: 89,
		common.MemoryKindOptionalValue:            41,
		common.MemoryKindNilValue:                 1,
		common.MemoryKindVoidValue:                1,
		common.MemoryKindTypeValue:                17,
		common.MemoryKindPathValue:                24,
		common.MemoryKindCapabilityValue:          1,
		common.MemoryKindLinkValue:                1,
		common.MemoryKindStorageReferenceValue:    41,
		common.MemoryKindEphemeralReferenceValue:  41,
		common.MemoryKindInterpretedFunctionValue: 128,
		common.MemoryKindHostFunctionValue:        41,
		common.MemoryKindBoundFunctionValue:       25,
		common.MemoryKindBigInt:                   50,

		// Atree

		common.MemoryKindAtreeArrayDataSlab:          80,
		common.MemoryKindAtreeArrayMetaDataSlab:      1024,
		common.MemoryKindAtreeArrayElementOverhead:   16,
		common.MemoryKindAtreeMapDataSlab:            144,
		common.MemoryKindAtreeMapMetaDataSlab:        1024,
		common.MemoryKindAtreeMapElementOverhead:     64,
		common.MemoryKindAtreeMapPreAllocatedElement: 24,
		common.MemoryKindAtreeEncodedSlab:            1536,

		// Static Types

		common.MemoryKindPrimitiveStaticType:     8,
		common.MemoryKindCompositeStaticType:     17,
		common.MemoryKindInterfaceStaticType:     17,
		common.MemoryKindVariableSizedStaticType: 17,
		common.MemoryKindConstantSizedStaticType: 25,
		common.MemoryKindDictionaryStaticType:    33,
		common.MemoryKindOptionalStaticType:      17,
		common.MemoryKindRestrictedStaticType:    41,
		common.MemoryKindReferenceStaticType:     41,
		common.MemoryKindCapabilityStaticType:    17,
		common.MemoryKindFunctionStaticType:      9,

		// Cadence Values

		common.MemoryKindCadenceVoidValue:         1,
		common.MemoryKindCadenceOptionalValue:     17,
		common.MemoryKindCadenceBoolValue:         8,
		common.MemoryKindCadenceStringValue:       16,
		common.MemoryKindCadenceCharacterValue:    16,
		common.MemoryKindCadenceAddressValue:      8,
		common.MemoryKindCadenceIntValue:          50,
		common.MemoryKindCadenceNumberValue:       1,
		common.MemoryKindCadenceArrayValueBase:    41,
		common.MemoryKindCadenceArrayValueLength:  16,
		common.MemoryKindCadenceDictionaryValue:   41,
		common.MemoryKindCadenceKeyValuePair:      33,
		common.MemoryKindCadenceStructValueBase:   33,
		common.MemoryKindCadenceStructValueSize:   16,
		common.MemoryKindCadenceResourceValueBase: 33,
		common.MemoryKindCadenceResourceValueSize: 16,
		common.MemoryKindCadenceEventValueBase:    33,
		common.MemoryKindCadenceEventValueSize:    16,
		common.MemoryKindCadenceContractValueBase: 33,
		common.MemoryKindCadenceContractValueSize: 16,
		common.MemoryKindCadenceEnumValueBase:     33,
		common.MemoryKindCadenceEnumValueSize:     16,
		common.MemoryKindCadenceLinkValue:         1,
		common.MemoryKindCadencePathValue:         33,
		common.MemoryKindCadenceTypeValue:         17,
		common.MemoryKindCadenceCapabilityValue:   1,

		// Cadence Types

		common.MemoryKindCadenceSimpleType:             1,
		common.MemoryKindCadenceOptionalType:           17,
		common.MemoryKindCadenceVariableSizedArrayType: 17,
		common.MemoryKindCadenceConstantSizedArrayType: 25,
		common.MemoryKindCadenceDictionaryType:         33,
		common.MemoryKindCadenceField:                  33,
		common.MemoryKindCadenceParameter:              49,
		common.MemoryKindCadenceStructType:             81,
		common.MemoryKindCadenceResourceType:           81,
		common.MemoryKindCadenceEventType:              81,
		common.MemoryKindCadenceContractType:           81,
		common.MemoryKindCadenceStructInterfaceType:    81,
		common.MemoryKindCadenceResourceInterfaceType:  81,
		common.MemoryKindCadenceContractInterfaceType:  81,
		common.MemoryKindCadenceFunctionType:           41,
		common.MemoryKindCadenceReferenceType:          25,
		common.MemoryKindCadenceRestrictedType:         57,
		common.MemoryKindCadenceCapabilityType:         17,
		common.MemoryKindCadenceEnumType:               97,

		// Misc

		common.MemoryKindRawString:         9,
		common.MemoryKindAddressLocation:   18,
		common.MemoryKindBytes:             24,
		common.MemoryKindVariable:          18,
		common.MemoryKindCompositeTypeInfo: 41,
		common.MemoryKindCompositeField:    33,
		common.MemoryKindInvocation:        89,
		common.MemoryKindStorageMap:        58,
		common.MemoryKindStorageKey:        41,

		// Tokens

		common.MemoryKindErrorToken: 41,
		common.MemoryKindTypeToken:  25,
		common.MemoryKindSpaceToken: 50,

		// AST nodes

		common.MemoryKindProgram:         220,
		common.MemoryKindIdentifier:      17,
		common.MemoryKindArgument:        49,
		common.MemoryKindBlock:           25,
		common.MemoryKindFunctionBlock:   25,
		common.MemoryKindParameter:       25,
		common.MemoryKindParameterList:   59,
		common.MemoryKindTransfer:        1,
		common.MemoryKindMembers:         276,
		common.MemoryKindTypeAnnotation:  25,
		common.MemoryKindDictionaryEntry: 33,

		common.MemoryKindFunctionDeclaration:        49,
		common.MemoryKindCompositeDeclaration:       65,
		common.MemoryKindInterfaceDeclaration:       41,
		common.MemoryKindEnumCaseDeclaration:        25,
		common.MemoryKindFieldDeclaration:           41,
		common.MemoryKindTransactionDeclaration:     81,
		common.MemoryKindImportDeclaration:          41,
		common.MemoryKindVariableDeclaration:        97,
		common.MemoryKindSpecialFunctionDeclaration: 17,
		common.MemoryKindPragmaDeclaration:          17,

		common.MemoryKindAssignmentStatement: 41,
		common.MemoryKindBreakStatement:      1,
		common.MemoryKindContinueStatement:   1,
		common.MemoryKindEmitStatement:       9,
		common.MemoryKindExpressionStatement: 17,
		common.MemoryKindForStatement:        33,
		common.MemoryKindIfStatement:         33,
		common.MemoryKindReturnStatement:     17,
		common.MemoryKindSwapStatement:       33,
		common.MemoryKindSwitchStatement:     41,
		common.MemoryKindWhileStatement:      25,

		common.MemoryKindBooleanExpression:     9,
		common.MemoryKindNilExpression:         1,
		common.MemoryKindStringExpression:      17,
		common.MemoryKindIntegerExpression:     33,
		common.MemoryKindFixedPointExpression:  49,
		common.MemoryKindArrayExpression:       25,
		common.MemoryKindDictionaryExpression:  25,
		common.MemoryKindIdentifierExpression:  1,
		common.MemoryKindInvocationExpression:  49,
		common.MemoryKindMemberExpression:      25,
		common.MemoryKindIndexExpression:       33,
		common.MemoryKindConditionalExpression: 49,
		common.MemoryKindUnaryExpression:       25,
		common.MemoryKindBinaryExpression:      41,
		common.MemoryKindFunctionExpression:    25,
		common.MemoryKindCastingExpression:     41,
		common.MemoryKindCreateExpression:      9,
		common.MemoryKindDestroyExpression:     17,
		common.MemoryKindReferenceExpression:   33,
		common.MemoryKindForceExpression:       17,
		common.MemoryKindPathExpression:        1,

		common.MemoryKindConstantSizedType: 25,
		common.MemoryKindDictionaryType:    33,
		common.MemoryKindFunctionType:      33,
		common.MemoryKindInstantiationType: 41,
		common.MemoryKindNominalType:       25,
		common.MemoryKindOptionalType:      17,
		common.MemoryKindReferenceType:     25,
		common.MemoryKindRestrictedType:    41,
		common.MemoryKindVariableSizedType: 17,

		common.MemoryKindPosition:          25,
		common.MemoryKindRange:             1,
		common.MemoryKindActivation:        128,
		common.MemoryKindActivationEntries: 256,
		common.MemoryKindElaboration:       501,

		// sema types
		common.MemoryKindVariableSizedSemaType: 51,
		common.MemoryKindConstantSizedSemaType: 59,
		common.MemoryKindDictionarySemaType:    67,
		common.MemoryKindOptionalSemaType:      17,
		common.MemoryKindRestrictedSemaType:    75,
		common.MemoryKindReferenceSemaType:     25,
		common.MemoryKindCapabilitySemaType:    51,

		// ordered-map
		common.MemoryKindOrderedMap:          17,
		common.MemoryKindOrderedMapEntryList: 50,
		common.MemoryKindOrderedMapEntry:     64,
	}
)

func _() {
	// A compiler error signifies that we have not accounted for all memory kinds
	var x [1]struct{}
	_ = x[int(common.MemoryKindLast)-len(DefaultMemoryWeights)-1]
}

type MeterParameters struct {
	computationLimit   uint64
	computationWeights ExecutionEffortWeights

	memoryLimit   uint64
	memoryWeights ExecutionMemoryWeights

	storageInteractionLimit uint64
	eventEmitByteLimit      uint64
}

func DefaultParameters() MeterParameters {
	// This is needed to work around golang's compiler bug
	umax := uint(math.MaxUint)
	return MeterParameters{
		computationLimit:        uint64(umax) << MeterExecutionInternalPrecisionBytes,
		memoryLimit:             math.MaxUint64,
		computationWeights:      DefaultComputationWeights,
		memoryWeights:           DefaultMemoryWeights,
		storageInteractionLimit: math.MaxUint64,
		eventEmitByteLimit:      math.MaxUint64,
	}
}

func (params MeterParameters) WithComputationLimit(limit uint) MeterParameters {
	newParams := params
	newParams.computationLimit = uint64(limit) << MeterExecutionInternalPrecisionBytes
	return newParams
}

func (params MeterParameters) WithComputationWeights(
	weights ExecutionEffortWeights,
) MeterParameters {
	newParams := params
	newParams.computationWeights = weights
	return newParams
}

func (params MeterParameters) WithMemoryLimit(limit uint64) MeterParameters {
	newParams := params
	newParams.memoryLimit = limit
	return newParams
}

func (params MeterParameters) WithMemoryWeights(
	weights ExecutionMemoryWeights,
) MeterParameters {
	newParams := params
	newParams.memoryWeights = weights
	return newParams
}

func (params MeterParameters) WithStorageInteractionLimit(
	maxStorageInteractionLimit uint64,
) MeterParameters {
	newParams := params
	newParams.storageInteractionLimit = maxStorageInteractionLimit
	return newParams
}

func (params MeterParameters) WithEventEmitByteLimit(
	byteLimit uint64,
) MeterParameters {
	newParams := params
	newParams.eventEmitByteLimit = byteLimit
	return newParams
}

func (params MeterParameters) ComputationWeights() ExecutionEffortWeights {
	return params.computationWeights
}

// TotalComputationLimit returns the total computation limit
func (params MeterParameters) TotalComputationLimit() uint {
	return uint(params.computationLimit >> MeterExecutionInternalPrecisionBytes)
}

func (params MeterParameters) MemoryWeights() ExecutionMemoryWeights {
	return params.memoryWeights
}

// TotalMemoryLimit returns the total memory limit
func (params MeterParameters) TotalMemoryLimit() uint64 {
	return params.memoryLimit
}

// Meter collects memory and computation usage and enforces limits
// for any each memory/computation usage call it sums intensity multiplied by the weight of the intensity to the total
// memory/computation usage metrics and returns error if limits are not met.
type Meter struct {
	MeterParameters

	computationUsed uint64
	memoryEstimate  uint64

	computationIntensities MeteredComputationIntensities
	memoryIntensities      MeteredMemoryIntensities

	storageUpdateSizeMap     map[StorageInteractionKey]uint64
	totalStorageBytesRead    uint64
	totalStorageBytesWritten uint64

	totalEmittedEventBytes uint64
}

type MeterOptions func(*Meter)

// NewMeter constructs a new Meter
func NewMeter(params MeterParameters) *Meter {
	m := &Meter{
		MeterParameters:        params,
		computationIntensities: make(MeteredComputationIntensities),
		memoryIntensities:      make(MeteredMemoryIntensities),
		storageUpdateSizeMap:   make(MeteredStorageInteractionMap),
	}

	return m
}

// NewChild construct a new Meter instance with the same limits as parent
func (m *Meter) NewChild() *Meter {
	return &Meter{
		MeterParameters:        m.MeterParameters,
		computationIntensities: make(MeteredComputationIntensities),
		memoryIntensities:      make(MeteredMemoryIntensities),
		storageUpdateSizeMap:   make(MeteredStorageInteractionMap),
	}
}

// MergeMeter merges the input meter into the current meter and checks for the limits
func (m *Meter) MergeMeter(child *Meter) {
	if child == nil {
		return
	}

	var childComputationUsed = child.computationUsed
	m.computationUsed = m.computationUsed + childComputationUsed

	for key, intensity := range child.computationIntensities {
		m.computationIntensities[key] += intensity
	}

	m.memoryEstimate = m.memoryEstimate + child.TotalMemoryEstimate()

	for key, intensity := range child.memoryIntensities {
		m.memoryIntensities[key] += intensity
	}

	// merge storage meters
	for key, value := range child.storageUpdateSizeMap {
		m.storageUpdateSizeMap[key] = value
	}
	m.totalStorageBytesRead += child.TotalBytesReadFromStorage()
	m.totalStorageBytesWritten += child.TotalBytesWrittenToStorage()

	m.totalEmittedEventBytes += child.TotalEmittedEventBytes()
}

// MeterComputation captures computation usage and returns an error if it goes beyond the limit
func (m *Meter) MeterComputation(kind common.ComputationKind, intensity uint) error {
	m.computationIntensities[kind] += intensity
	w, ok := m.computationWeights[kind]
	if !ok {
		return nil
	}
	m.computationUsed += w * uint64(intensity)
	if m.computationUsed > m.computationLimit {
		return errors.NewComputationLimitExceededError(uint64(m.TotalComputationLimit()))
	}
	return nil
}

// ComputationIntensities returns all the measured computational intensities
func (m *Meter) ComputationIntensities() MeteredComputationIntensities {
	return m.computationIntensities
}

// TotalComputationUsed returns the total computation used
func (m *Meter) TotalComputationUsed() uint {
	return uint(m.computationUsed >> MeterExecutionInternalPrecisionBytes)
}

// MeterMemory captures memory usage and returns an error if it goes beyond the limit
func (m *Meter) MeterMemory(kind common.MemoryKind, intensity uint) error {
	m.memoryIntensities[kind] += intensity
	w, ok := m.memoryWeights[kind]
	if !ok {
		return nil
	}
	m.memoryEstimate += w * uint64(intensity)
	if m.memoryEstimate > m.memoryLimit {
		return errors.NewMemoryLimitExceededError(uint64(m.TotalMemoryLimit()))
	}
	return nil
}

// MemoryIntensities returns all the measured memory intensities
func (m *Meter) MemoryIntensities() MeteredMemoryIntensities {
	return m.memoryIntensities
}

// TotalMemoryEstimate returns the total memory used
func (m *Meter) TotalMemoryEstimate() uint64 {
	return m.memoryEstimate
}

// MeterStorageRead captures storage read bytes count and returns an error
// if it goes beyond the total interaction limit and limit is enforced
func (m *Meter) MeterStorageRead(
	storageKey StorageInteractionKey,
	value flow.RegisterValue,
	enforceLimit bool) error {

	// all reads are on a View which only read from storage at the first read of a given key
	if _, ok := m.storageUpdateSizeMap[storageKey]; !ok {
		readByteSize := getStorageKeyValueSize(storageKey, value)
		m.totalStorageBytesRead += readByteSize
		m.storageUpdateSizeMap[storageKey] = readByteSize
	}

	return m.checkStorageInteractionLimit(enforceLimit)
}

// MeterStorageRead captures storage written bytes count and returns an error
// if it goes beyond the total interaction limit and limit is enforced
func (m *Meter) MeterStorageWrite(
	storageKey StorageInteractionKey,
	value flow.RegisterValue,
	enforceLimit bool) error {
	// all writes are on a View which only writes the latest updated value to storage at commit
	if old, ok := m.storageUpdateSizeMap[storageKey]; ok {
		m.totalStorageBytesWritten -= old
	}

	updateSize := getStorageKeyValueSize(storageKey, value)
	m.totalStorageBytesWritten += updateSize
	m.storageUpdateSizeMap[storageKey] = updateSize

	return m.checkStorageInteractionLimit(enforceLimit)
}

func (m *Meter) checkStorageInteractionLimit(enforceLimit bool) error {
	if enforceLimit &&
		m.TotalBytesOfStorageInteractions() > m.storageInteractionLimit {
		return errors.NewLedgerInteractionLimitExceededError(
			m.TotalBytesOfStorageInteractions(), m.storageInteractionLimit)
	}
	return nil
}

// TotalBytesReadFromStorage returns total number of byte read from storage
func (m *Meter) TotalBytesReadFromStorage() uint64 {
	return m.totalStorageBytesRead
}

// TotalBytesReadFromStorage returns total number of byte written to storage
func (m *Meter) TotalBytesWrittenToStorage() uint64 {
	return m.totalStorageBytesWritten
}

// TotalBytesOfStorageInteractions returns total number of byte read and written from/to storage
func (m *Meter) TotalBytesOfStorageInteractions() uint64 {
	return m.TotalBytesReadFromStorage() + m.TotalBytesWrittenToStorage()
}

func getStorageKeyValueSize(storageKey StorageInteractionKey,
	value flow.RegisterValue) uint64 {
	return uint64(len(storageKey.Owner) + len(storageKey.Key) + len(value))
}

func GetStorageKeyValueSizeForTesting(
	storageKey StorageInteractionKey,
	value flow.RegisterValue) uint64 {
	return getStorageKeyValueSize(storageKey, value)
}

func (m *Meter) GetStorageUpdateSizeMapForTesting() MeteredStorageInteractionMap {
	return m.storageUpdateSizeMap
}

func (m *Meter) MeterEmittedEvent(byteSize uint64) error {
	m.totalEmittedEventBytes += uint64(byteSize)

	if m.totalEmittedEventBytes > m.eventEmitByteLimit {
		return errors.NewEventLimitExceededError(
			m.totalEmittedEventBytes,
			m.eventEmitByteLimit)
	}
	return nil
}

func (m *Meter) TotalEmittedEventBytes() uint64 {
	return m.totalEmittedEventBytes
}
