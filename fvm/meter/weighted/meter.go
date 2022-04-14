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

		common.MemoryKindBool:                    0,
		common.MemoryKindAddress:                 0,
		common.MemoryKindString:                  0,
		common.MemoryKindCharacter:               0,
		common.MemoryKindMetaType:                0,
		common.MemoryKindNumber:                  0,
		common.MemoryKindArrayBase:               0,
		common.MemoryKindArrayLength:             0,
		common.MemoryKindDictionaryBase:          0,
		common.MemoryKindDictionarySize:          0,
		common.MemoryKindCompositeBase:           0,
		common.MemoryKindCompositeSize:           0,
		common.MemoryKindOptional:                0,
		common.MemoryKindNil:                     0,
		common.MemoryKindVoid:                    0,
		common.MemoryKindTypeValue:               0,
		common.MemoryKindPathValue:               0,
		common.MemoryKindCapabilityValue:         0,
		common.MemoryKindLinkValue:               0,
		common.MemoryKindStorageReferenceValue:   0,
		common.MemoryKindEphemeralReferenceValue: 0,
		common.MemoryKindInterpretedFunction:     0,
		common.MemoryKindHostFunction:            0,
		common.MemoryKindBoundFunction:           0,
		common.MemoryKindBigInt:                  0,

		// Misc

		common.MemoryKindRawString:       0,
		common.MemoryKindAddressLocation: 0,
		common.MemoryKindBytes:           0,
		common.MemoryKindVariable:        0,

		// Tokens

		common.MemoryKindTokenIdentifier:     0,
		common.MemoryKindTokenComment:        0,
		common.MemoryKindTokenNumericLiteral: 0,
		common.MemoryKindTokenSyntax:         0,

		// AST nodes

		common.MemoryKindProgram:        0,
		common.MemoryKindIdentifier:     0,
		common.MemoryKindArgument:       0,
		common.MemoryKindBlock:          0,
		common.MemoryKindFunctionBlock:  0,
		common.MemoryKindParameter:      0,
		common.MemoryKindParameterList:  0,
		common.MemoryKindTransfer:       0,
		common.MemoryKindMembers:        0,
		common.MemoryKindTypeAnnotation: 0,

		common.MemoryKindFunctionDeclaration:        0,
		common.MemoryKindCompositeDeclaration:       0,
		common.MemoryKindInterfaceDeclaration:       0,
		common.MemoryKindEnumCaseDeclaration:        0,
		common.MemoryKindFieldDeclaration:           0,
		common.MemoryKindTransactionDeclaration:     0,
		common.MemoryKindImportDeclaration:          0,
		common.MemoryKindVariableDeclaration:        0,
		common.MemoryKindSpecialFunctionDeclaration: 0,
		common.MemoryKindPragmaDeclaration:          0,

		common.MemoryKindAssignmentStatement: 0,
		common.MemoryKindBreakStatement:      0,
		common.MemoryKindContinueStatement:   0,
		common.MemoryKindEmitStatement:       0,
		common.MemoryKindExpressionStatement: 0,
		common.MemoryKindForStatement:        0,
		common.MemoryKindIfStatement:         0,
		common.MemoryKindReturnStatement:     0,
		common.MemoryKindSwapStatement:       0,
		common.MemoryKindSwitchStatement:     0,
		common.MemoryKindWhileStatement:      0,

		common.MemoryKindBooleanExpression:     0,
		common.MemoryKindNilExpression:         0,
		common.MemoryKindStringExpression:      0,
		common.MemoryKindIntegerExpression:     0,
		common.MemoryKindFixedPointExpression:  0,
		common.MemoryKindArrayExpression:       0,
		common.MemoryKindDictionaryExpression:  0,
		common.MemoryKindIdentifierExpression:  0,
		common.MemoryKindInvocationExpression:  0,
		common.MemoryKindMemberExpression:      0,
		common.MemoryKindIndexExpression:       0,
		common.MemoryKindConditionalExpression: 0,
		common.MemoryKindUnaryExpression:       0,
		common.MemoryKindBinaryExpression:      0,
		common.MemoryKindFunctionExpression:    0,
		common.MemoryKindCastingExpression:     0,
		common.MemoryKindCreateExpression:      0,
		common.MemoryKindDestroyExpression:     0,
		common.MemoryKindReferenceExpression:   0,
		common.MemoryKindForceExpression:       0,
		common.MemoryKindPathExpression:        0,

		common.MemoryKindConstantSizedType: 0,
		common.MemoryKindDictionaryType:    0,
		common.MemoryKindFunctionType:      0,
		common.MemoryKindInstantiationType: 0,
		common.MemoryKindNominalType:       0,
		common.MemoryKindOptionalType:      0,
		common.MemoryKindReferenceType:     0,
		common.MemoryKindRestrictedType:    0,
		common.MemoryKindVariableSizedType: 0,

		common.MemoryKindPosition: 0,
		common.MemoryKindRange:    0,
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
	if int(common.MemoryKindLast)-len(weights) != 1 {
		return func(m *Meter) {
			m.memoryWeights = weights
		}
	}

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
	}
	m.memoryWeights = weights
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
