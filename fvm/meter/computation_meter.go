package meter

import (
	"math"

	"github.com/onflow/cadence/common"

	"github.com/onflow/flow-go/fvm/errors"
)

type MeteredComputationIntensities map[common.ComputationKind]uint64

var (
	// DefaultComputationWeights is the default weights for computation intensities
	// these weighs make the computation metering the same as it was before dynamic execution fees
	// these weighs make the computation metering the same as it was before dynamic execution fees
	DefaultComputationWeights = ExecutionEffortWeights{
		common.ComputationKindStatement:          1 << MeterExecutionInternalPrecisionBytes,
		common.ComputationKindLoop:               1 << MeterExecutionInternalPrecisionBytes,
		common.ComputationKindFunctionInvocation: 1 << MeterExecutionInternalPrecisionBytes,
	}
)

// MeterExecutionInternalPrecisionBytes are the amount of bytes that are used internally by the
// WeigthedMeter to allow for metering computation smaller than one unit of computation.
// This allows for more fine weights. A weight of 1 unit of computation is equal to 1<<16.
// The minimum possible weight is 1/65536.
const MeterExecutionInternalPrecisionBytes = 16

type ExecutionEffortWeights map[common.ComputationKind]uint64

func (weights ExecutionEffortWeights) ComputationFromIntensities(intensities MeteredComputationIntensities) uint64 {
	var result uint64
	for kind, weight := range weights {
		intensity := uint64(intensities[kind])
		result += weight * intensity
	}
	return result >> MeterExecutionInternalPrecisionBytes
}

type ComputationMeterParameters struct {
	computationLimit   uint64
	computationWeights ExecutionEffortWeights
}

func DefaultComputationMeterParameters() ComputationMeterParameters {
	return ComputationMeterParameters{
		computationLimit:   math.MaxUint64,
		computationWeights: DefaultComputationWeights,
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

func (params ComputationMeterParameters) ComputationWeights() ExecutionEffortWeights {
	return params.computationWeights
}

// TotalComputationLimit returns the total computation limit
func (params ComputationMeterParameters) TotalComputationLimit() uint64 {
	return params.computationLimit >> MeterExecutionInternalPrecisionBytes
}

type ComputationMeter struct {
	params ComputationMeterParameters

	computationUsed        uint64
	computationIntensities MeteredComputationIntensities
}

func NewComputationMeter(params ComputationMeterParameters) ComputationMeter {
	return ComputationMeter{
		params:                 params,
		computationIntensities: make(MeteredComputationIntensities),
	}
}

// MeterComputation captures computation usage and returns an error if it goes beyond the limit
func (m *ComputationMeter) MeterComputation(usage common.ComputationUsage) error {
	kind := usage.Kind
	intensity := usage.Intensity

	m.computationIntensities[kind] += intensity
	w, ok := m.params.computationWeights[kind]
	if !ok {
		return nil
	}
	m.computationUsed += w * intensity
	if m.computationUsed > m.params.computationLimit {
		return errors.NewComputationLimitExceededError(
			uint64(m.params.TotalComputationLimit()))
	}
	return nil
}

// ComputationAvailable returns true if enough computation is left in the transaction for the given intensity and type
func (m *ComputationMeter) ComputationAvailable(usage common.ComputationUsage) bool {
	w, ok := m.params.computationWeights[usage.Kind]
	// if not found return has capacity
	// given the behaviour of MeterComputation is ignoring intensities without a set weight
	if !ok {
		return true
	}

	potentialComputationUsage := m.computationUsed + w*usage.Intensity
	return potentialComputationUsage <= m.params.computationLimit
}

// ComputationRemaining returns the remaining computation (intensity) left in the transaction for the given type
func (m *ComputationMeter) ComputationRemaining(kind common.ComputationKind) uint64 {
	w, ok := m.params.computationWeights[kind]
	// if not found return has capacity
	// given the behaviour of MeterComputation is ignoring intensities without a set weight
	if !ok {
		return math.MaxUint64
	}

	remainingComputationUsage := m.params.computationLimit - m.computationUsed
	if remainingComputationUsage <= 0 {
		return 0
	}

	return remainingComputationUsage / w
}

// ComputationIntensities returns all the measured computational intensities
func (m *ComputationMeter) ComputationIntensities() MeteredComputationIntensities {
	return m.computationIntensities
}

// TotalComputationUsed returns the total computation used
func (m *ComputationMeter) TotalComputationUsed() uint64 {
	return m.computationUsed >> MeterExecutionInternalPrecisionBytes
}

func (m *ComputationMeter) Merge(child ComputationMeter) {
	m.computationUsed = m.computationUsed + child.computationUsed

	for key, intensity := range child.computationIntensities {
		m.computationIntensities[key] += intensity
	}
}
