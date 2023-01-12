package environment

import (
	"encoding/binary"
	"math/rand"
	"sync"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

type UnsafeRandomGenerator interface {
	// UnsafeRandom returns a random uint64, where the process of random number
	// derivation is not cryptographically secure.
	UnsafeRandom() (uint64, error)
}

type unsafeRandomGenerator struct {
	tracer tracing.TracerSpan

	blockHeader *flow.Header

	rng      *rand.Rand
	seedOnce sync.Once
}

type ParseRestrictedUnsafeRandomGenerator struct {
	txnState *state.TransactionState
	impl     UnsafeRandomGenerator
}

func NewParseRestrictedUnsafeRandomGenerator(
	txnState *state.TransactionState,
	impl UnsafeRandomGenerator,
) UnsafeRandomGenerator {
	return ParseRestrictedUnsafeRandomGenerator{
		txnState: txnState,
		impl:     impl,
	}
}

func (gen ParseRestrictedUnsafeRandomGenerator) UnsafeRandom() (
	uint64,
	error,
) {
	return parseRestrict1Ret(
		gen.txnState,
		trace.FVMEnvUnsafeRandom,
		gen.impl.UnsafeRandom)
}

func NewUnsafeRandomGenerator(
	tracer tracing.TracerSpan,
	blockHeader *flow.Header,
) UnsafeRandomGenerator {
	gen := &unsafeRandomGenerator{
		tracer:      tracer,
		blockHeader: blockHeader,
	}

	return gen
}

// seed seeds the random number generator with the block header ID.
// This allows lazy seeding of the random number generator,
// since not a lot of transactions/scripts use it and the time it takes to seed it is not negligible.
func (gen *unsafeRandomGenerator) seed() {
	gen.seedOnce.Do(func() {
		if gen.blockHeader == nil {
			return
		}
		// Seed the random number generator with entropy created from the block
		// header ID. The random number generator will be used by the
		// UnsafeRandom function.
		id := gen.blockHeader.ID()
		source := rand.NewSource(int64(binary.BigEndian.Uint64(id[:])))
		gen.rng = rand.New(source)
	})
}

// UnsafeRandom returns a random uint64, where the process of random number
// derivation is not cryptographically secure.
// this is not thread safe, due to gen.rng.Read(buf).
// Its also not thread safe because each thread needs to be deterministically seeded with a different seed.
// This is Ok because a single transaction has a single UnsafeRandomGenerator and is run in a single thread.
func (gen *unsafeRandomGenerator) UnsafeRandom() (uint64, error) {
	defer gen.tracer.StartExtensiveTracingChildSpan(trace.FVMEnvUnsafeRandom).End()

	gen.seed()

	if gen.rng == nil {
		return 0, errors.NewOperationNotSupportedError("UnsafeRandom")
	}

	// TODO (ramtin) return errors this assumption that this always succeeds
	// might not be true
	buf := make([]byte, 8)
	_, _ = gen.rng.Read(buf) // Always succeeds, no need to check error
	return binary.LittleEndian.Uint64(buf), nil
}
