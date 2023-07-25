package environment

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/module/trace"
)

type BlockEntropyProvider interface {
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

func (b *blockEntropyProvider) BlockEntropy() ([]byte, error) {
	defer b.tracer.StartExtensiveTracingChildSpan(
		trace.FVMEnvBlockEntropyProvider).End()

	err := b.meter.MeterComputation(ComputationKindGetBlockEntropy, 1)
	if err != nil {
		return nil, fmt.Errorf("get block entropy failed: %w", err)
	}

	source, err := b.entropyProvider.RandomSource()
	if err != nil {
		return nil, fmt.Errorf(
			"get random source for block entropy failed: %w", err)
	}

	// TODO: Tarak, convert this to correct bytes

	return source, nil
}
