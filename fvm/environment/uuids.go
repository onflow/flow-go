package environment

import (
	"encoding/binary"
	"fmt"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/slices"
)

const keyUUID = "uuid"

type UUIDGenerator struct {
	tracer *Tracer
	meter  Meter

	txnState *state.TransactionState
}

func NewUUIDGenerator(
	tracer *Tracer,
	meter Meter,
	txnState *state.TransactionState,
) *UUIDGenerator {
	return &UUIDGenerator{
		tracer:   tracer,
		meter:    meter,
		txnState: txnState,
	}
}

// GetUUID reads uint64 byte value for uuid from the state
func (generator *UUIDGenerator) GetUUID() (uint64, error) {
	stateBytes, err := generator.txnState.Get(
		"",
		keyUUID,
		generator.txnState.EnforceLimits())
	if err != nil {
		return 0, fmt.Errorf("cannot get uuid byte from state: %w", err)
	}
	bytes := slices.EnsureByteSliceSize(stateBytes, 8)

	return binary.BigEndian.Uint64(bytes), nil
}

// SetUUID sets a new uint64 byte value
func (generator *UUIDGenerator) SetUUID(uuid uint64) error {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, uuid)
	err := generator.txnState.Set(
		"",
		keyUUID,
		bytes,
		generator.txnState.EnforceLimits())
	if err != nil {
		return fmt.Errorf("cannot set uuid byte to state: %w", err)
	}
	return nil
}

// GenerateUUID generates a new uuid and persist the data changes into state
func (generator *UUIDGenerator) GenerateUUID() (uint64, error) {
	defer generator.tracer.StartExtensiveTracingSpanFromRoot(
		trace.FVMEnvGenerateUUID).End()

	err := generator.meter.MeterComputation(
		ComputationKindGenerateUUID,
		1)
	if err != nil {
		return 0, fmt.Errorf("generate uuid failed: %w", err)
	}

	uuid, err := generator.GetUUID()
	if err != nil {
		return 0, fmt.Errorf("cannot generate UUID: %w", err)
	}

	err = generator.SetUUID(uuid + 1)
	if err != nil {
		return 0, fmt.Errorf("cannot generate UUID: %w", err)
	}
	return uuid, nil
}
