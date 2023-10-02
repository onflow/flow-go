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
	"github.com/onflow/flow-go/module/metrics"
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
	metrics *metrics.ExecutionCollector,
	chainID flow.ChainID,
	entropy query.EntropyProviderPerBlock,
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
		metrics,
		vm,
		vmCtx,
		derivedChainData,
		entropy,
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

	snap, header, err := s.snapshotWithBlock(height)
	if err != nil {
		return nil, err
	}

	return s.executor.ExecuteScript(ctx, script, arguments, header, snap)
}

func (s *Scripts) GetAccount(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	snap, header, err := s.snapshotWithBlock(height)
	if err != nil {
		return nil, err
	}

	return s.executor.GetAccount(ctx, address, header, snap)
}

func (s *Scripts) snapshotWithBlock(height uint64) (snapshot.StorageSnapshot, *flow.Header, error) {
	header, err := s.headers.ByHeight(height)
	if err != nil {
		return nil, nil, err
	}

	storageSnapshot := snapshot.NewReadFuncStorageSnapshot(func(ID flow.RegisterID) (flow.RegisterValue, error) {
		values, err := s.registersAtHeight(flow.RegisterIDs{ID}, height)
		if err != nil {
			return nil, err
		}
		if len(values) > 1 || len(values) == 0 {
			return nil, fmt.Errorf("invalid number of returned values for a single register: %d", len(values))
		}
		return values[0], nil
	})

	return storageSnapshot, header, nil
}
