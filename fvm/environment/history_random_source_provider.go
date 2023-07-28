package environment

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol/prg"
)

type RandomSourceHistoryProvider interface {
	// RandomSourceHistory provides a source of entropy that can be
	// expanded on-chain into randoms (using a pseudo-random generator).
	// This random source is only destined to the history source core-contract
	// to implement commit-reveal schemes.
	// The returned slice should have at least 128 bits of entropy.
	// The function doesn't error in normal operations, any
	// error should be treated as an exception.
	RandomSourceHistory() ([]byte, error)
}

type ParseRestrictedRandomSourceHistoryProvider struct {
	txnState state.NestedTransactionPreparer
	impl     RandomSourceHistoryProvider
}

func NewParseRestrictedRandomSourceHistoryProvider(
	txnState state.NestedTransactionPreparer,
	impl RandomSourceHistoryProvider,
) RandomSourceHistoryProvider {
	return ParseRestrictedRandomSourceHistoryProvider{
		txnState: txnState,
		impl:     impl,
	}
}

func (p ParseRestrictedRandomSourceHistoryProvider) RandomSourceHistory() ([]byte, error) {
	return parseRestrict1Ret(
		p.txnState,
		trace.FVMEnvRandomSourceHistoryProvider,
		p.impl.RandomSourceHistory,
	)
}

// forbiddenRandomSourceHistoryProvider is a RandomSourceHistoryProvider that always returns an error.
// this is the default implementation of RandomSourceHistoryProvider.
type forbiddenRandomSourceHistoryProvider struct {
}

func NewForbiddenRandomSourceHistoryProvider() RandomSourceHistoryProvider {
	return &forbiddenRandomSourceHistoryProvider{}
}

func (b forbiddenRandomSourceHistoryProvider) RandomSourceHistory() ([]byte, error) {
	return nil, errors.NewOperationNotSupportedError("RandomSourceHistory")
}

type historySourceProvider struct {
	tracer tracing.TracerSpan
	meter  Meter
	EntropyProvider
}

// NewRandomSourceHistoryProvider creates a new RandomSourceHistoryProvider.
// If randomSourceCallAllowed is true, the returned RandomSourceHistoryProvider will
// return a random source from the given EntropyProvider.
// If randomSourceCallAllowed is false, the returned RandomSourceHistoryProvider will
// always return an error.
func NewRandomSourceHistoryProvider(
	tracer tracing.TracerSpan,
	meter Meter,
	entropyProvider EntropyProvider,
	randomSourceCallAllowed bool,
) RandomSourceHistoryProvider {
	if randomSourceCallAllowed {
		return &historySourceProvider{
			tracer:          tracer,
			meter:           meter,
			EntropyProvider: entropyProvider,
		}
	}

	return NewForbiddenRandomSourceHistoryProvider()
}

const randomSourceHistoryLen = 32

func (b *historySourceProvider) RandomSourceHistory() ([]byte, error) {
	defer b.tracer.StartExtensiveTracingChildSpan(
		trace.FVMEnvRandomSourceHistoryProvider).End()

	err := b.meter.MeterComputation(ComputationKindGetRandomSourceHistory, 1)
	if err != nil {
		return nil, fmt.Errorf("get block randomSource failed: %w", err)
	}

	source, err := b.RandomSource()
	// `RandomSource` does not error in normal operations.
	// Any error should be treated as an exception.
	if err != nil {
		return nil, errors.NewRandomSourceFailure(fmt.Errorf(
			"get random source for block randomSource failed: %w", err))
	}

	// A method that derives `randomSourceHistoryLen` bytes from `source` must:
	//  - extract and expand the entropy in `source`
	//  - output must be independent than the expanded bytes used for Cadence's `random` function
	//
	// The method chosen here is to rely on the same CSPRG used to derive randoms from the source entropy
	// (but other methods are possible)
	//  - use the state/protocol/prg customizer defined for the execution random source history.
	//   (to ensure independence of seeds, customizer must be different than the one used for Cadence's
	//     `random` in random_generator.go)
	csprg, err := prg.New(source, prg.ExecutionRandomSourceHistory, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create a PRG from source: %w", err)
	}

	historySource := make([]byte, randomSourceHistoryLen)
	csprg.Read(historySource)

	return historySource, nil
}
