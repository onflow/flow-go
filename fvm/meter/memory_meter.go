package meter

import (
	"math"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
)

var (
	// DefaultMemoryWeights are currently hard-coded here. In the future we might like to
	// define this in a contract similar to the computation weights
	DefaultMemoryWeights = ExecutionMemoryWeights{

		// Values

		common.MemoryKindAddressValue:   32,
		common.MemoryKindStringValue:    138,
		common.MemoryKindCharacterValue: 24,
		common.MemoryKindNumberValue:    8,
		// weights for these values include the cost of the Go struct itself (first number)
		// as well as the overhead for creation of the underlying atree (second number)
		common.MemoryKindArrayValueBase:                   57 + 48,
		common.MemoryKindDictionaryValueBase:              33 + 96,
		common.MemoryKindCompositeValueBase:               233 + 96,
		common.MemoryKindSimpleCompositeValue:             73,
		common.MemoryKindSimpleCompositeValueBase:         89,
		common.MemoryKindOptionalValue:                    41,
		common.MemoryKindTypeValue:                        17,
		common.MemoryKindPathValue:                        24,
		common.MemoryKindPathCapabilityValue:              1,
		common.MemoryKindIDCapabilityValue:                1,
		common.MemoryKindPathLinkValue:                    1,
		common.MemoryKindStorageCapabilityControllerValue: 32,
		common.MemoryKindAccountCapabilityControllerValue: 32,
		common.MemoryKindAccountLinkValue:                 1,
		common.MemoryKindAccountReferenceValue:            1,
		common.MemoryKindPublishedValue:                   1,
		common.MemoryKindStorageReferenceValue:            41,
		common.MemoryKindEphemeralReferenceValue:          41,
		common.MemoryKindInterpretedFunctionValue:         128,
		common.MemoryKindHostFunctionValue:                41,
		common.MemoryKindBoundFunctionValue:               25,
		common.MemoryKindBigInt:                           50,
		common.MemoryKindVoidExpression:                   1,

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

		common.MemoryKindCadenceVoidValue:           1,
		common.MemoryKindCadenceOptionalValue:       17,
		common.MemoryKindCadenceBoolValue:           8,
		common.MemoryKindCadenceStringValue:         16,
		common.MemoryKindCadenceCharacterValue:      16,
		common.MemoryKindCadenceAddressValue:        8,
		common.MemoryKindCadenceIntValue:            50,
		common.MemoryKindCadenceNumberValue:         1,
		common.MemoryKindCadenceArrayValueBase:      41,
		common.MemoryKindCadenceArrayValueLength:    16,
		common.MemoryKindCadenceDictionaryValue:     41,
		common.MemoryKindCadenceKeyValuePair:        33,
		common.MemoryKindCadenceStructValueBase:     33,
		common.MemoryKindCadenceStructValueSize:     16,
		common.MemoryKindCadenceResourceValueBase:   33,
		common.MemoryKindCadenceResourceValueSize:   16,
		common.MemoryKindCadenceEventValueBase:      33,
		common.MemoryKindCadenceEventValueSize:      16,
		common.MemoryKindCadenceContractValueBase:   33,
		common.MemoryKindCadenceContractValueSize:   16,
		common.MemoryKindCadenceEnumValueBase:       33,
		common.MemoryKindCadenceEnumValueSize:       16,
		common.MemoryKindCadencePathLinkValue:       1,
		common.MemoryKindCadenceAccountLinkValue:    1,
		common.MemoryKindCadencePathValue:           33,
		common.MemoryKindCadenceTypeValue:           17,
		common.MemoryKindCadencePathCapabilityValue: 1,
		common.MemoryKindCadenceIDCapabilityValue:   1,
		common.MemoryKindCadenceFunctionValue:       1,
		common.MemoryKindCadenceAttachmentValueBase: 33,
		common.MemoryKindCadenceAttachmentValueSize: 16,

		// Cadence Types

		common.MemoryKindCadenceTypeParameter:          17,
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
		common.MemoryKindCadenceAttachmentType:         81,

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

		common.MemoryKindProgram:           220,
		common.MemoryKindIdentifier:        17,
		common.MemoryKindArgument:          49,
		common.MemoryKindBlock:             25,
		common.MemoryKindFunctionBlock:     25,
		common.MemoryKindParameter:         25,
		common.MemoryKindParameterList:     59,
		common.MemoryKindTypeParameter:     9,
		common.MemoryKindTypeParameterList: 59,
		common.MemoryKindTransfer:          1,
		common.MemoryKindMembers:           276,
		common.MemoryKindTypeAnnotation:    25,
		common.MemoryKindDictionaryEntry:   33,

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
		common.MemoryKindAttachmentDeclaration:      70,

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
		common.MemoryKindRemoveStatement:     33,

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
		common.MemoryKindAttachExpression:      33,

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

type ExecutionMemoryWeights map[common.MemoryKind]uint64
type MeteredMemoryIntensities map[common.MemoryKind]uint

type MemoryMeterParameters struct {
	memoryLimit   uint64
	memoryWeights ExecutionMemoryWeights
}

func DefaultMemoryParameters() MemoryMeterParameters {
	return MemoryMeterParameters{
		memoryLimit:   math.MaxUint64,
		memoryWeights: DefaultMemoryWeights,
	}
}

func (params MemoryMeterParameters) MemoryWeights() ExecutionMemoryWeights {
	return params.memoryWeights
}

// TotalMemoryLimit returns the total memory limit
func (params MemoryMeterParameters) TotalMemoryLimit() uint64 {
	return params.memoryLimit
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

type MemoryMeter struct {
	params MemoryMeterParameters

	memoryIntensities MeteredMemoryIntensities
	memoryEstimate    uint64
}

// MemoryIntensities returns all the measured memory intensities
func (m *MemoryMeter) MemoryIntensities() MeteredMemoryIntensities {
	return m.memoryIntensities
}

// NewMemoryMeter constructs a new Meter
func NewMemoryMeter(params MemoryMeterParameters) MemoryMeter {
	m := MemoryMeter{
		params:            params,
		memoryIntensities: make(MeteredMemoryIntensities),
	}

	return m
}

// MeterMemory captures memory usage and returns an error if it goes beyond the limit
func (m *MemoryMeter) MeterMemory(kind common.MemoryKind, intensity uint) error {
	m.memoryIntensities[kind] += intensity
	w, ok := m.params.memoryWeights[kind]
	if !ok {
		return nil
	}
	m.memoryEstimate += w * uint64(intensity)
	if m.memoryEstimate > m.params.memoryLimit {
		return errors.NewMemoryLimitExceededError(m.params.TotalMemoryLimit())
	}
	return nil
}

// TotalMemoryEstimate returns the total memory used
func (m *MemoryMeter) TotalMemoryEstimate() uint64 {
	return m.memoryEstimate
}

// Merge merges the input meter into the current meter and checks for the limits
func (m *MemoryMeter) Merge(child MemoryMeter) {
	m.memoryEstimate = m.memoryEstimate + child.TotalMemoryEstimate()

	for key, intensity := range child.memoryIntensities {
		m.memoryIntensities[key] += intensity
	}
}
