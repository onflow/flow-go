package environment

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"sync"

	"golang.org/x/crypto/hkdf"

	"github.com/onflow/flow-go/crypto/random"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/state"
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

	prg      random.Rand
	seedOnce sync.Once
}

type ParseRestrictedUnsafeRandomGenerator struct {
	txnState state.NestedTransactionPreparer
	impl     UnsafeRandomGenerator
}

func NewParseRestrictedUnsafeRandomGenerator(
	txnState state.NestedTransactionPreparer,
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

// This function abstracts building the PRG seed from the entropy source `randomSource`.
// It does not make assumptions about the quality of the source, nor about
// its length (the source could be a fingerprint of entity, an ID of an entity,
//
//	a beacon signature..)
//
// It therefore uses a mechansim to extract the source entropy and expand it into
// the required `seedLen` bytes (this can be a KDF, a MAC, a hash with extended-length output..)
func seedFromEntropySource(randomSource []byte, seedLen int) ([]byte, error) {
	// This implementation used HKDF,
	// but other promitives with the 2 properties above could also be used.
	hkdf := hkdf.New(func() hash.Hash { return sha256.New() }, randomSource, nil, nil)
	seed := make([]byte, random.Chacha20SeedLen)
	n, err := hkdf.Read(seed)
	if n != len(seed) {
		return nil, fmt.Errorf("extracting seed with HKDF failed, required %d bytes, got %d", random.Chacha20SeedLen, n)
	}
	if err != nil {
		return nil, fmt.Errorf("extracting seed with HKDF failed: %w", err)
	}
	return seed, nil
}

// seed seeds the pseudo-random number generator using the block header ID
// as an entropy source.
// The seed function is currently called for each tranaction, the PRG is used
// to provide all the randoms the transaction needs through UnsafeRandom.
//
// This allows lazy seeding of the random number generator,
// since not a lot of transactions/scripts use it and the time it takes to seed it is not negligible.
func (gen *unsafeRandomGenerator) seed() {
	gen.seedOnce.Do(func() {
		if gen.blockHeader == nil {
			return
		}

		// The block header ID is currently used as the entropy source.
		// This should evolve to become the beacon signature (safer entropy source than
		// the block ID)
		// Extract the entropy from the source and expand it into the required seed length.
		source := gen.blockHeader.ID()
		seed, err := seedFromEntropySource(source[:], random.Chacha20SeedLen)
		if err != nil {
			return
		}

		// initialize a fresh crypto-secure PRG with the seed (here ChaCha20)
		// This PRG provides all outputs of Cadence UnsafeRandom.
		prg, err := random.NewChacha20PRG(seed, []byte{})
		if err != nil {
			return
		}
		gen.prg = prg
	})
}

// UnsafeRandom returns a random uint64 using the underlying PRG (currently using a crypto-secure one).
// this is not thread safe, due to the gen.prg instance currently used.
// Its also not thread safe because each thread needs to be deterministically seeded with a different seed.
// This is Ok because a single transaction has a single UnsafeRandomGenerator and is run in a single thread.
func (gen *unsafeRandomGenerator) UnsafeRandom() (uint64, error) {
	defer gen.tracer.StartExtensiveTracingChildSpan(trace.FVMEnvUnsafeRandom).End()

	// The internal seeding is only done once.
	gen.seed()

	if gen.prg == nil {
		return 0, errors.NewOperationNotSupportedError("UnsafeRandom")
	}

	buf := make([]byte, 8)
	gen.prg.Read(buf)
	return binary.LittleEndian.Uint64(buf), nil
}
