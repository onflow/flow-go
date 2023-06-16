package environment

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"sync"

	"golang.org/x/crypto/hkdf"

	"github.com/onflow/flow-go/crypto/random"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
)

type UnsafeRandomGenerator interface {
	// UnsafeRandom returns a random uint64
	UnsafeRandom() (uint64, error)
}

type unsafeRandomGenerator struct {
	tracer tracing.TracerSpan

	stateSnapshot protocol.Snapshot
	txId          flow.Identifier

	prg        random.Rand
	createOnce sync.Once
	createErr  error
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
	stateSnapshot protocol.Snapshot,
	txId flow.Identifier,
) UnsafeRandomGenerator {
	gen := &unsafeRandomGenerator{
		tracer:        tracer,
		stateSnapshot: stateSnapshot,
		txId:          txId,
	}

	return gen
}

func (gen *unsafeRandomGenerator) createRandomGenerator() (
	random.Rand,
	error,
) {
	if gen.stateSnapshot == nil {
		return nil, nil
	}

	// Use the protocol state source of randomness for the currrent block
	// execution (which is the randmoness beacon output for thiss block)
	source, err := gen.stateSnapshot.RandomSource()
	// expected errors of RandomSource() are :
	// - storage.ErrNotFound if the QC is unknown.
	// - state.ErrUnknownSnapshotReference if the snapshot reference block is unknown
	// at this stage, snapshot reference block should be known and the QC should also be known,
	// so no error is expected in normal operations
	if err != nil {
		return nil, fmt.Errorf("reading random source from state failed: %w", err)
	}

	// Diversify the seed per transaction ID
	salt := gen.txId[:]

	// Extract the entropy from the source and expand it into the required
	// seed length.  Note that we can use any implementation which provide
	// similar properties.
	hkdf := hkdf.New(
		func() hash.Hash { return sha256.New() },
		source[:],
		salt,
		nil)
	seed := make([]byte, random.Chacha20SeedLen)
	_, err = io.ReadFull(hkdf, seed)
	if err != nil {
		return nil, fmt.Errorf("extracting seed with HKDF failed: %w", err)
	}

	// initialize a fresh crypto-secure PRG with the seed (here ChaCha20)
	// This PRG provides all outputs of Cadence UnsafeRandom.
	prg, err := random.NewChacha20PRG(seed, []byte{})
	if err != nil {
		return nil, fmt.Errorf("creating random generator failed: %w", err)
	}

	return prg, nil
}

// maybeCreateRandomGenerator seeds the pseudo-random number generator using the
// block header ID and transaction index as an entropy source.  The seed
// function is currently called for each tranaction, the PRG is used to
// provide all the randoms the transaction needs through UnsafeRandom.
//
// This allows lazy seeding of the random number generator, since not a lot of
// transactions/scripts use it and the time it takes to seed it is not
// negligible.
func (gen *unsafeRandomGenerator) maybeCreateRandomGenerator() error {
	gen.createOnce.Do(func() {
		gen.prg, gen.createErr = gen.createRandomGenerator()
	})

	return gen.createErr
}

// UnsafeRandom returns a random uint64 using the underlying PRG (currently
// using a crypto-secure one).  This is not thread safe, due to the gen.prg
// instance currently used.  Its also not thread safe because each thread needs
// to be deterministically seeded with a different seed.  This is Ok because a
// single transaction has a single UnsafeRandomGenerator and is run in a single
// thread.
func (gen *unsafeRandomGenerator) UnsafeRandom() (uint64, error) {
	defer gen.tracer.StartExtensiveTracingChildSpan(
		trace.FVMEnvUnsafeRandom).End()

	// The internal seeding is only done once.
	err := gen.maybeCreateRandomGenerator()
	if err != nil {
		return 0, err
	}

	if gen.prg == nil {
		return 0, errors.NewOperationNotSupportedError("UnsafeRandom")
	}

	buf := make([]byte, 8)
	gen.prg.Read(buf) // Note: prg.Read does not return error
	return binary.LittleEndian.Uint64(buf), nil
}
