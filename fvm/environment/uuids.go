package environment

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/slices"
)

// uuid is partitioned with 3rd byte for compatibility reasons.
// (database types and Javascript safe integer limits)
//
// counter(C) is 7 bytes, paritition(P) is 1 byte
// C7 C6 P C5 C4 C3 C2 C1
//
// Until resource ids start filling the bits above the 48th one, dapps will have enough time
// to switch to a larger data type.

const (
	// The max value for any is uuid partition is MaxUint56, since one byte
	// in the uuid is used for partitioning.
	MaxUint56 = (uint64(1) << 56) - 1

	// Start warning when there's only a single high bit left.  This should give
	// us plenty of time to migrate to larger counters.
	Uint56OverflowWarningThreshold = (uint64(1) << 55) - 1
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
	log    zerolog.Logger
	meter  Meter

	txnState state.NestedTransactionPreparer

	blockHeader *flow.Header
	txnIndex    uint32

	initialized bool
	partition   byte
	registerId  flow.RegisterID
}

func uuidPartition(blockId flow.Identifier, txnIndex uint32) byte {
	// Partitioning by txnIndex ensures temporally neighboring transactions do
	// not share registers / conflict with each other.
	//
	// Since all blocks will have a transaction at txnIndex 0 but not
	// necessarily a transaction at txnIndex 255, if we assign partition based
	// only on txnIndex, partition 0's counter (and other low-valued
	// partitions' counters) will fill up much more quickly than high-valued
	// partitions' counters.  Therefore, a deterministically random offset is
	// used to ensure the partitioned counters are roughly balanced.  Any byte
	// in the sha hash is sufficiently random/uniform for this purpose (Note that
	// block Id is already a sha hash, but its hash implementation may change
	// underneath us).
	//
	// Note that since partition 0 reuses the legacy counter, its counter is
	// much	further ahead than the other partitions.  If partition 0's counter
	// is in danager of overflowing, use variants of "the power of two random
	// choices" to shed load to other counters.
	//
	// The explicit mod is not really needed, but is there for completeness.
	partitionOffset := sha256.Sum256(blockId[:])[0]
	return byte((uint32(partitionOffset) + txnIndex) % 256)
}

func NewUUIDGenerator(
	tracer tracing.TracerSpan,
	log zerolog.Logger,
	meter Meter,
	txnState state.NestedTransactionPreparer,
	blockHeader *flow.Header,
	txnIndex uint32,
) *uUIDGenerator {
	return &uUIDGenerator{
		tracer:      tracer,
		log:         log,
		meter:       meter,
		txnState:    txnState,
		blockHeader: blockHeader,
		txnIndex:    txnIndex,
		initialized: false,
	}
}

// getUint64 reads the uint64 value from the partitioned uuid register.
func (generator *uUIDGenerator) getUint64() (uint64, error) {
	stateBytes, err := generator.txnState.Get(generator.registerId)
	if err != nil {
		return 0, fmt.Errorf(
			"cannot get uuid partition %d byte from state: %w",
			generator.partition,
			err)
	}
	bytes := slices.EnsureByteSliceSize(stateBytes, 8)

	return binary.BigEndian.Uint64(bytes), nil
}

// setUint56 sets a new uint56 value into the partitioned uuid register.
func (generator *uUIDGenerator) setUint56(
	value uint64,
) error {
	if value > Uint56OverflowWarningThreshold {
		if value > MaxUint56 {
			return fmt.Errorf(
				"uuid partition %d overflowed",
				generator.partition)
		}

		generator.log.Warn().
			Int("partition", int(generator.partition)).
			Uint64("value", value).
			Msg("uuid partition is running out of bits")
	}

	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, value)
	err := generator.txnState.Set(generator.registerId, bytes)
	if err != nil {
		return fmt.Errorf(
			"cannot set uuid %d byte to state: %w",
			generator.partition,
			err)
	}
	return nil
}

func (generator *uUIDGenerator) maybeInitializePartition() {
	if generator.initialized {
		return
	}
	generator.initialized = true

	// NOTE: block header is not set for scripts.  We'll just use partition 0 in
	// this case.
	if generator.blockHeader == nil {
		generator.partition = 0
	} else {
		generator.partition = uuidPartition(
			generator.blockHeader.ID(),
			generator.txnIndex)
	}

	generator.registerId = flow.UUIDRegisterID(generator.partition)
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

	generator.maybeInitializePartition()

	value, err := generator.getUint64()
	if err != nil {
		return 0, fmt.Errorf("cannot generate UUID: %w", err)
	}

	err = generator.setUint56(value + 1)
	if err != nil {
		return 0, fmt.Errorf("cannot generate UUID: %w", err)
	}

	// Since the partition counter only goes up to MaxUint56, we can use the
	// 6th byte to represent which partition was used.
	// (C7 C6) | P | (C5 C4 C3 C2 C1)
	return ((value & 0xFF_FF00_0000_0000) << 8) | (uint64(generator.partition) << 40) | (value & 0xFF_FFFF_FFFF), nil

}
