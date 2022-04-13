package weighted

import (
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	interfaceMeter "github.com/onflow/flow-go/fvm/meter"
)

// MeterExecutionInternalPrecisionBytes are the amount of bytes that are used internally by the Meter
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

		common.MemoryKindBool:                    8,
		common.MemoryKindAddress:                 32,
		common.MemoryKindString:                  138,
		common.MemoryKindCharacter:               24,
		common.MemoryKindMetaType:                0,
		common.MemoryKindNumber:                  8,
		common.MemoryKindArrayBase:               57,
		common.MemoryKindArrayLength:             1024,
		common.MemoryKindDictionaryBase:          33,
		common.MemoryKindDictionarySize:          1024,
		common.MemoryKindCompositeBase:           233,
		common.MemoryKindCompositeSize:           1024,
		common.MemoryKindOptional:                41,
		common.MemoryKindNil:                     1,
		common.MemoryKindVoid:                    1,
		common.MemoryKindTypeValue:               0,
		common.MemoryKindPathValue:               24,
		common.MemoryKindCapabilityValue:         1,
		common.MemoryKindLinkValue:               1,
		common.MemoryKindStorageReferenceValue:   41,
		common.MemoryKindEphemeralReferenceValue: 41,
		common.MemoryKindInterpretedFunction:     128,
		common.MemoryKindHostFunction:            41,
		common.MemoryKindBoundFunction:           25,
		common.MemoryKindBigInt:                  50,

		// Misc

		common.MemoryKindRawString:       9,
		common.MemoryKindAddressLocation: 18,
		common.MemoryKindBytes:           24,
		common.MemoryKindVariable:        18,

		// Tokens

		common.MemoryKindTokenIdentifier:     0,
		common.MemoryKindTokenComment:        0,
		common.MemoryKindTokenNumericLiteral: 0,
		common.MemoryKindTokenSyntax:         25,

		// AST nodes

		common.MemoryKindProgram:        220,
		common.MemoryKindIdentifier:     17,
		common.MemoryKindArgument:       49,
		common.MemoryKindBlock:          25,
		common.MemoryKindFunctionBlock:  25,
		common.MemoryKindParameter:      25,
		common.MemoryKindParameterList:  59,
		common.MemoryKindTransfer:       1,
		common.MemoryKindMembers:        276,
		common.MemoryKindTypeAnnotation: 25,

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

		common.MemoryKindPosition: 25,
		common.MemoryKindRange:    1,
	}
)

func _() {
	// A compiler error signifies that we have not accounted for all memory kinds
	var x [1]struct{}
	_ = x[int(common.MemoryKindLast)-len(DefaultMemoryWeights)-1]
}

var _ interfaceMeter.Meter = &Meter{}

// Meter collects memory and computation usage and enforces limits
// for any each memory/computation usage call it sums intensity multiplied by the weight of the intensity to the total
// memory/computation usage metrics and returns error if limits are not met.
type Meter struct {
	computationUsed  uint64
	computationLimit uint64
	memoryUsed       uint64
	memoryLimit      uint64

	computationIntensities interfaceMeter.MeteredComputationIntensities
	memoryIntensities      interfaceMeter.MeteredMemoryIntensities

	computationWeights ExecutionEffortWeights
	memoryWeights      ExecutionMemoryWeights
}

type WeightedMeterOptions func(*Meter)

// NewMeter constructs a new Meter
func NewMeter(computationLimit, memoryLimit uint, options ...WeightedMeterOptions) *Meter {

	m := &Meter{
		computationLimit:       uint64(computationLimit) << MeterExecutionInternalPrecisionBytes,
		memoryLimit:            uint64(memoryLimit),
		computationWeights:     DefaultComputationWeights,
		memoryWeights:          DefaultMemoryWeights,
		computationIntensities: make(interfaceMeter.MeteredComputationIntensities),
		memoryIntensities:      make(interfaceMeter.MeteredMemoryIntensities),
	}

	for _, option := range options {
		option(m)
	}

	return m
}

// WithComputationWeights sets the weights for computation intensities
func WithComputationWeights(weights ExecutionEffortWeights) WeightedMeterOptions {
	return func(m *Meter) {
		m.computationWeights = weights
	}
}

// WithMemoryWeights sets the weights for the memory intensities
func WithMemoryWeights(weights ExecutionMemoryWeights) WeightedMeterOptions {
	// Replace some weights
	if int(common.MemoryKindLast)-len(weights) != 1 {
		return func(m *Meter) {
			for kind, weight := range weights {
				m.memoryWeights[kind] = weight
			}
		}
	}

	// Replace all weights
	return func(m *Meter) {
		m.memoryWeights = weights
	}
}

// NewChild construct a new Meter instance with the same limits as parent
func (m *Meter) NewChild() interfaceMeter.Meter {
	return &Meter{
		computationLimit:       m.computationLimit,
		memoryLimit:            m.memoryLimit,
		computationWeights:     m.computationWeights,
		memoryWeights:          m.memoryWeights,
		computationIntensities: make(interfaceMeter.MeteredComputationIntensities),
		memoryIntensities:      make(interfaceMeter.MeteredMemoryIntensities),
	}
}

// MergeMeter merges the input meter into the current meter and checks for the limits
func (m *Meter) MergeMeter(child interfaceMeter.Meter) error {

	var childComputationUsed uint64
	if basic, ok := child.(*Meter); ok {
		childComputationUsed = basic.computationUsed
	} else {
		childComputationUsed = uint64(child.TotalComputationUsed()) << MeterExecutionInternalPrecisionBytes
	}
	m.computationUsed = m.computationUsed + childComputationUsed
	if m.computationUsed > m.computationLimit {
		return errors.NewComputationLimitExceededError(m.computationLimit)
	}

	for key, intensity := range child.ComputationIntensities() {
		m.computationIntensities[key] += intensity
	}

	var childMemoryUsed uint64
	if basic, ok := child.(*Meter); ok {
		childMemoryUsed = basic.memoryUsed
	} else {
		childMemoryUsed = uint64(child.TotalMemoryUsed())
	}
	m.memoryUsed = m.memoryUsed + childMemoryUsed
	if m.memoryUsed > m.memoryLimit {
		return errors.NewMemoryLimitExceededError(m.memoryLimit)
	}

	for key, intensity := range child.MemoryIntensities() {
		m.memoryIntensities[key] += intensity
	}
	return nil
}

// SetComputationWeights sets the computation weights
func (m *Meter) SetComputationWeights(weights ExecutionEffortWeights) {
	m.computationWeights = weights
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
		return errors.NewComputationLimitExceededError(m.computationLimit)
	}
	return nil
}

// ComputationIntensities returns all the measured computational intensities
func (m *Meter) ComputationIntensities() interfaceMeter.MeteredComputationIntensities {
	return m.computationIntensities
}

// TotalComputationUsed returns the total computation used
func (m *Meter) TotalComputationUsed() uint {
	return uint(m.computationUsed >> MeterExecutionInternalPrecisionBytes)
}

// TotalComputationLimit returns the total computation limit
func (m *Meter) TotalComputationLimit() uint {
	return uint(m.computationLimit >> MeterExecutionInternalPrecisionBytes)
}

// SetMemoryWeights sets the memory weights
func (m *Meter) SetMemoryWeights(weights ExecutionMemoryWeights) {
	if int(common.MemoryKindLast)-len(weights) != 1 {
		for kind, weight := range weights {
			m.memoryWeights[kind] = weight
		}
	} else {
		m.memoryWeights = weights
	}
}

// MeterMemory captures memory usage and returns an error if it goes beyond the limit
func (m *Meter) MeterMemory(kind common.MemoryKind, intensity uint) error {
	m.memoryIntensities[kind] += intensity
	w, ok := m.memoryWeights[kind]
	if !ok {
		return nil
	}
	m.memoryUsed += w * uint64(intensity)
	if m.memoryUsed > m.memoryLimit {
		return errors.NewMemoryLimitExceededError(m.memoryLimit)
	}
	return nil
}

// MemoryIntensities returns all the measured memory intensities
func (m *Meter) MemoryIntensities() interfaceMeter.MeteredMemoryIntensities {
	return m.memoryIntensities
}

// TotalMemoryUsed returns the total memory used
func (m *Meter) TotalMemoryUsed() uint {
	return uint(m.memoryUsed)
}

// TotalMemoryLimit returns the total memory limit
func (m *Meter) TotalMemoryLimit() uint {
	return uint(m.memoryLimit)
}
