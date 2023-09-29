package execution

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// RegistersAtHeight returns register values for provided register IDs at the block height.
// Even if the register wasn't indexed at the provided height, returns the highest height the register was indexed at.
// Expected errors:
// - storage.ErrNotFound if the register by the ID was never indexed
// - ErrIndexBoundary if the height is out of indexed height boundary
type RegistersAtHeight func(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error)

type Scripts struct {
	executor          *query.QueryExecutor
	headers           storage.Headers
	registersAtHeight RegistersAtHeight
}

func NewScripts(
	log zerolog.Logger,
	tracer module.Tracer,
	chainID flow.ChainID,
	state protocol.State,
	header storage.Headers,
	registersAtHeight RegistersAtHeight,
) (*Scripts, error) {
	vm := fvm.NewVirtualMachine()

	options := computation.DefaultFVMOptions(chainID, false, false)
	vmCtx := fvm.NewContext(options...)

	derivedChainData, err := derived.NewDerivedChainData(derived.DefaultDerivedDataCacheSize)
	if err != nil {
		return nil, err
	}

	queryExecutor := query.NewQueryExecutor(
		query.NewDefaultConfig(),
		log,
		metrics.NewExecutionCollector(tracer),
		vm,
		vmCtx,
		derivedChainData,
		query.NewProtocolStateWrapper(state),
	)

	return &Scripts{
		executor:          queryExecutor,
		headers:           header,
		registersAtHeight: registersAtHeight,
	}, nil
}

// ExecuteAtBlockHeight executes provided script against the block height.
// A result value is returned encoded as byte array. An error will be returned if script
// doesn't successfully execute.
// Expected errors:
// - Storage.NotFound if block at height was not found.
func (s *Scripts) ExecuteAtBlockHeight(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
	height uint64) ([]byte, error) {

	header, err := s.headers.ByHeight(height)
	if err != nil {
		return nil, err
	}

	storageSnapshot := snapshot.NewReadFuncStorageSnapshot(func(ID flow.RegisterID) (flow.RegisterValue, error) {
		values, err := s.registersAtHeight(flow.RegisterIDs{ID}, height)
		if err != nil {
			return nil, err
		}
		if len(values) > 1 {
			return nil, fmt.Errorf("multiple values returned for a single register")
		}
		return values[0], nil
	})

	return s.executor.ExecuteScript(ctx, script, arguments, header, storageSnapshot)
}
