package environment

import (
	"fmt"

	"github.com/onflow/crypto/random"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol/prg"
)

// EntropyProvider represents an entropy (source of randomness) provider
type EntropyProvider interface {
	// RandomSource provides a source of entropy that can be
	// expanded into randoms (using a pseudo-random generator).
	// The returned slice should have at least 128 bits of entropy.
	// The function doesn't error in normal operations, any
	// error should be treated as an exception.
	RandomSource() ([]byte, error)
}

type RandomGenerator interface {
	// ReadRandom reads pseudo-random bytes into the input slice, using distributed randomness.
	// The name follows Cadence interface
	ReadRandom([]byte) error
}

var _ RandomGenerator = (*randomGenerator)(nil)

// randomGenerator implements RandomGenerator and is used
// for the transactions execution environment
type randomGenerator struct {
	tracer        tracing.TracerSpan
	entropySource EntropyProvider
	salt          []byte
	prg           random.Rand
	isPRGCreated  bool
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

func (gen ParseRestrictedRandomGenerator) ReadRandom(buf []byte) error {
	return parseRestrict1Arg(
		gen.txnState,
		trace.FVMEnvRandom,
		gen.impl.ReadRandom,
		buf)
}

func NewRandomGenerator(
	tracer tracing.TracerSpan,
	entropySource EntropyProvider,
	salt []byte,
) RandomGenerator {
	gen := &randomGenerator{
		tracer:        tracer,
		entropySource: entropySource,
		salt:          salt,
		isPRGCreated:  false, // PRG is not created
	}

	return gen
}

func (gen *randomGenerator) createPRG() (random.Rand, error) {
	// Use the protocol state source of randomness [SoR] for the current block's
	// execution
	source, err := gen.entropySource.RandomSource()
	// `RandomSource` does not error in normal operations.
	// Any error should be treated as an exception.
	if err != nil {
		return nil, fmt.Errorf("reading random source from state failed: %w", err)
	}

	// Use the state/protocol PRG derivation from the source of randomness:
	//  - for the transaction execution case, the PRG used must be a CSPRG
	//  - use the state/protocol/prg customizer defined for the execution environment
	//  - use the salt as an extra diversifier of the CSPRG. Although this
	//    does not add any extra entropy to the output, it allows creating an independent
	//    PRG for each transaction or script.
	csprg, err := prg.New(source, prg.ExecutionEnvironment, gen.salt)
	if err != nil {
		return nil, fmt.Errorf("failed to create a CSPRG from source: %w", err)
	}

	return csprg, nil
}

// ReadRandom reads pseudo-random bytes into the input slice using the underlying PRG (currently
// using a crypto-secure one). This function is not thread safe, due to the gen.prg
// instance currently used. This is fine because a
// single transaction has a single RandomGenerator and is run in a single
// thread.
func (gen *randomGenerator) ReadRandom(buf []byte) error {
	defer gen.tracer.StartExtensiveTracingChildSpan(
		trace.FVMEnvRandom).End()

	// PRG creation is only done once.
	if !gen.isPRGCreated {
		newPRG, err := gen.createPRG()
		if err != nil {
			return err
		}
		gen.prg = newPRG
		gen.isPRGCreated = true
	}

	gen.prg.Read(buf)
	return nil
}
