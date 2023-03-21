package environment

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"sync"

	"golang.org/x/crypto/hkdf"

	"github.com/onflow/flow-go/crypto/random"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

type UnsafeRandomGenerator interface {
	// UnsafeRandom returns a random uint64
	UnsafeRandom() (uint64, error)
}

type unsafeRandomGenerator struct {
	tracer tracing.TracerSpan

	blockHeader *flow.Header

	rng      random.Rand
	seedOnce sync.Once
}

type ParseRestrictedUnsafeRandomGenerator struct {
	txnState state.NestedTransaction
	impl     UnsafeRandomGenerator
}

func NewParseRestrictedUnsafeRandomGenerator(
	txnState state.NestedTransaction,
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
		// extract the entropy from `id` and expand it into the required seed
		hkdf := hkdf.New(func() hash.Hash { return sha256.New() }, id[:], nil, nil)
		seed := make([]byte, random.Chacha20SeedLen)
		n, err := hkdf.Read(seed)
		if n != len(seed) || err != nil {
			return
		}
		// initialize a fresh CSPRNG with the seed (crypto-secure PRG)
		source, err := random.NewChacha20PRG(seed, []byte{})
		if err != nil {
			return
		}
		gen.rng = source
	})
}

// UnsafeRandom returns a random uint64 using the underlying PRG (currently using a crypto-secure one).
// this is not thread safe, due to the gen.rng instance currently used.
// Its also not thread safe because each thread needs to be deterministically seeded with a different seed.
// This is Ok because a single transaction has a single UnsafeRandomGenerator and is run in a single thread.
func (gen *unsafeRandomGenerator) UnsafeRandom() (uint64, error) {
	defer gen.tracer.StartExtensiveTracingChildSpan(trace.FVMEnvUnsafeRandom).End()

	gen.seed()

	if gen.rng == nil {
		return 0, errors.NewOperationNotSupportedError("UnsafeRandom")
	}

	buf := make([]byte, 8)
	gen.rng.Read(buf)
	return binary.LittleEndian.Uint64(buf), nil
}
