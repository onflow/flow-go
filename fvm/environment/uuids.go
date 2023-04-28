package environment

import (
	"encoding/binary"
	"fmt"

	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/slices"
)

type UUIDGenerator interface {
	GenerateUUID() (uint64, error)
}

type ParseRestrictedUUIDGenerator struct {
	txnState state.NestedTransactionPreparer
	impl     UUIDGenerator
}

func NewParseRestrictedUUIDGenerator(
	txnState state.NestedTransactionPreparer,
	impl UUIDGenerator,
) UUIDGenerator {
	return ParseRestrictedUUIDGenerator{
		txnState: txnState,
		impl:     impl,
	}
}

func (generator ParseRestrictedUUIDGenerator) GenerateUUID() (uint64, error) {
	return parseRestrict1Ret(
		generator.txnState,
		trace.FVMEnvGenerateUUID,
		generator.impl.GenerateUUID)
}

type uUIDGenerator struct {
	tracer tracing.TracerSpan
	meter  Meter

	txnState state.NestedTransactionPreparer
}

func NewUUIDGenerator(
	tracer tracing.TracerSpan,
	meter Meter,
	txnState state.NestedTransactionPreparer,
) *uUIDGenerator {
	return &uUIDGenerator{
		tracer:   tracer,
		meter:    meter,
		txnState: txnState,
	}
}

// GetUUID reads uint64 byte value for uuid from the state
func (generator *uUIDGenerator) getUUID() (uint64, error) {
	stateBytes, err := generator.txnState.Get(flow.UUIDRegisterID)
	if err != nil {
		return 0, fmt.Errorf("cannot get uuid byte from state: %w", err)
	}
	bytes := slices.EnsureByteSliceSize(stateBytes, 8)

	return binary.BigEndian.Uint64(bytes), nil
}

// SetUUID sets a new uint64 byte value
func (generator *uUIDGenerator) setUUID(uuid uint64) error {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, uuid)
	err := generator.txnState.Set(flow.UUIDRegisterID, bytes)
	if err != nil {
		return fmt.Errorf("cannot set uuid byte to state: %w", err)
	}
	return nil
}

// GenerateUUID generates a new uuid and persist the data changes into state
func (generator *uUIDGenerator) GenerateUUID() (uint64, error) {
	defer generator.tracer.StartExtensiveTracingChildSpan(
		trace.FVMEnvGenerateUUID).End()

	err := generator.meter.MeterComputation(
		ComputationKindGenerateUUID,
		1)
	if err != nil {
		return 0, fmt.Errorf("generate uuid failed: %w", err)
	}

	uuid, err := generator.getUUID()
	if err != nil {
		return 0, fmt.Errorf("cannot generate UUID: %w", err)
	}

	err = generator.setUUID(uuid + 1)
	if err != nil {
		return 0, fmt.Errorf("cannot generate UUID: %w", err)
	}
	return uuid, nil
}
