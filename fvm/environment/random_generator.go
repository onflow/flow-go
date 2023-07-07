package environment

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/crypto/random"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/prg"
)

type RandomGenerator interface {
	// UnsafeRandom returns a random uint64
	// Todo: rename to Random() once Cadence interface is updated
	UnsafeRandom() (uint64, error)
}

var _ RandomGenerator = (*randomGenerator)(nil)

// randomGenerator implements RandomGenerator and is used
// for the transactions execution environment
type randomGenerator struct {
	tracer tracing.TracerSpan

	stateSnapshot protocol.Snapshot
	txId          flow.Identifier

	prg        random.Rand
	createOnce sync.Once
	createErr  error
}

type ParseRestrictedRandomGenerator struct {
	txnState state.NestedTransactionPreparer
	impl     RandomGenerator
}

func NewParseRestrictedRandomGenerator(
	txnState state.NestedTransactionPreparer,
	impl RandomGenerator,
) RandomGenerator {
	return ParseRestrictedRandomGenerator{
		txnState: txnState,
		impl:     impl,
	}
}

func (gen ParseRestrictedRandomGenerator) UnsafeRandom() (
	uint64,
	error,
) {
	return parseRestrict1Ret(
		gen.txnState,
		trace.FVMEnvRandom,
		gen.impl.UnsafeRandom)
}

func NewRandomGenerator(
	tracer tracing.TracerSpan,
	stateSnapshot protocol.Snapshot,
	txId flow.Identifier,
) RandomGenerator {
	gen := &randomGenerator{
		tracer:        tracer,
		stateSnapshot: stateSnapshot,
		txId:          txId,
	}

	return gen
}

func (gen *randomGenerator) createRandomGenerator() (
	random.Rand,
	error,
) {
	// Use the protocol state source of randomness [SoR] for the current block's
	// execution
	source, err := gen.stateSnapshot.RandomSource()
	// expected errors of RandomSource() are:
	// - storage.ErrNotFound if the QC is unknown.
	// - state.ErrUnknownSnapshotReference if the snapshot reference block is unknown
	// at this stage, snapshot reference block should be known and the QC should also be known,
	// so no error is expected in normal operations
	if err != nil {
		return nil, fmt.Errorf("reading random source from state failed: %w", err)
	}

	// Use the state/protocol PRG derivation from the source of randomness:
	//  - for the transaction execution case, the PRG used must be a CSPRG
	//  - use the state/protocol/prg customizer defined for the execution environment
	//  - use the transaction ID as an extra diversifier of the CSPRG. Although this
	//    does not add any extra entropy to the output, it allows creating an independent
	//    PRG for each transaction.
	csprg, err := prg.New(source, prg.ExecutionEnvironment, gen.txId[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create a CSPRG from source: %w", err)
	}

	return csprg, nil
}

// maybeCreateRandomGenerator seeds the pseudo-random number generator using the
// block SoR as an entropy source, customized with the transaction hash. The seed
// function is currently called for each transaction, the PRG is used to
// provide all the randoms the transaction needs through Random.
//
// This allows lazy seeding of the random number generator, since not a lot of
// transactions/scripts use it and the time it takes to seed it is not
// negligible.
func (gen *randomGenerator) maybeCreateRandomGenerator() error {
	gen.createOnce.Do(func() {
		gen.prg, gen.createErr = gen.createRandomGenerator()
	})

	return gen.createErr
}

// Random returns a random uint64 using the underlying PRG (currently
// using a crypto-secure one).  This is not thread safe, due to the gen.prg
// instance currently used.  Its also not thread safe because each thread needs
// to be deterministically seeded with a different seed.  This is Ok because a
// single transaction has a single RandomGenerator and is run in a single
// thread.
func (gen *randomGenerator) UnsafeRandom() (uint64, error) {
	defer gen.tracer.StartExtensiveTracingChildSpan(
		trace.FVMEnvRandom).End()

	// The internal seeding is only done once.
	err := gen.maybeCreateRandomGenerator()
	if err != nil {
		return 0, err
	}

	buf := make([]byte, 8)
	gen.prg.Read(buf) // Note: prg.Read does not return error
	return binary.LittleEndian.Uint64(buf), nil
}

var _ RandomGenerator = (*dummyRandomGenerator)(nil)

// dummyRandomGenerator implements RandomGenerator and is used
// for the scripts execution environment
type dummyRandomGenerator struct{}

func NewDummyRandomGenerator() RandomGenerator {
	return &dummyRandomGenerator{}
}

// Random() returns an error because executing scripts
// does not support randomness APIs.
func (gen *dummyRandomGenerator) UnsafeRandom() (uint64, error) {
	return 0, errors.NewOperationNotSupportedError("Random")
}
