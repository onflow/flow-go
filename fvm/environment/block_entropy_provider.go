package environment

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol/prg"
)

type BlockEntropyProvider interface {
	// RandomSource provides a source of entropy that can be
	// expanded into randoms (using a pseudo-random generator).
	// The returned slice should have at least 128 bits of entropy.
	// The function doesn't error in normal operations, any
	// error should be treated as an exception.
	BlockEntropy() ([]byte, error)
}

type ParseRestrictedBlockEntropyProvider struct {
	txnState state.NestedTransactionPreparer
	impl     BlockEntropyProvider
}

func NewParseRestrictedBlockEntropyProvider(
	txnState state.NestedTransactionPreparer,
	impl BlockEntropyProvider,
) BlockEntropyProvider {
	return ParseRestrictedBlockEntropyProvider{
		txnState: txnState,
		impl:     impl,
	}
}

func (p ParseRestrictedBlockEntropyProvider) BlockEntropy() ([]byte, error) {
	return parseRestrict1Ret(
		p.txnState,
		trace.FVMEnvBlockEntropyProvider,
		p.impl.BlockEntropy,
	)
}

// forbiddenBlockEntropyProvider is a BlockEntropyProvider that always returns an error.
// this is the default implementation of BlockEntropyProvider.
type forbiddenBlockEntropyProvider struct {
}

func NewForbiddenBlockEntropyProvider() BlockEntropyProvider {
	return &forbiddenBlockEntropyProvider{}
}

func (b forbiddenBlockEntropyProvider) BlockEntropy() ([]byte, error) {
	return nil, errors.NewOperationNotSupportedError("BlockEntropy")
}

type blockEntropyProvider struct {
	tracer          tracing.TracerSpan
	meter           Meter
	entropyProvider EntropyProvider
}

// NewBlockEntropyProvider creates a new BlockEntropyProvider.
// If blockEntropyCallAllowed is true, the returned BlockEntropyProvider will
// return a random source from the given EntropyProvider.
// If blockEntropyCallAllowed is false, the returned BlockEntropyProvider will
// always return an error.
func NewBlockEntropyProvider(
	tracer tracing.TracerSpan,
	meter Meter,
	entropyProvider EntropyProvider,
	blockEntropyCallAllowed bool,
) BlockEntropyProvider {
	if blockEntropyCallAllowed {
		return &blockEntropyProvider{
			tracer:          tracer,
			meter:           meter,
			entropyProvider: entropyProvider,
		}
	}

	return NewForbiddenBlockEntropyProvider()
}

const historyRandomSourceLen = 32

func (b *blockEntropyProvider) BlockEntropy() ([]byte, error) {
	defer b.tracer.StartExtensiveTracingChildSpan(
		trace.FVMEnvBlockEntropyProvider).End()

	err := b.meter.MeterComputation(ComputationKindGetBlockEntropy, 1)
	if err != nil {
		return nil, fmt.Errorf("get block entropy failed: %w", err)
	}

	source, err := b.entropyProvider.RandomSource()
	// `RandomSource` does not error in normal operations.
	// Any error should be treated as an exception.
	if err != nil {
		return nil, fmt.Errorf(
			"get random source for block entropy failed: %w", err)
	}

	// A method that derives `historyRandomSourceLen` bytes from `source` must:
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

	historySource := make([]byte, historyRandomSourceLen)
	csprg.Read(historySource)

	return historySource, nil
}
