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

// RegistersAtHeight returns register value for provided register ID at the block height.
// Even if the register wasn't indexed at the provided height, returns the highest height the register was indexed at.
// Expected errors:
// - storage.ErrNotFound if the register by the ID was never indexed
// - ErrIndexBoundary if the height is out of indexed height boundary
type RegistersAtHeight func(ID flow.RegisterID, height uint64) (flow.RegisterValue, error)

type ScriptExecutor interface {
	// ExecuteAtBlockHeight executes provided script against the block height.
	// A result value is returned encoded as byte array. An error will be returned if script
	// doesn't successfully execute.
	// Expected errors:
	// - storage.ErrNotFound if block or register value at height was not found.
	ExecuteAtBlockHeight(
		ctx context.Context,
		script []byte,
		arguments [][]byte,
		height uint64,
	) ([]byte, error)

	// GetAccountAtBlockHeight returns a Flow account by the provided address and block height.
	// Expected errors:
	// - storage.ErrNotFound if block or register value at height was not found.
	GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error)
}

var _ ScriptExecutor = (*Scripts)(nil)

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
// - storage.ErrNotFound if block or register value at height was not found.
func (s *Scripts) ExecuteAtBlockHeight(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
	height uint64,
) ([]byte, error) {

	snap, header, err := s.snapshotWithBlock(height)
	if err != nil {
		return nil, err
	}

	return s.executor.ExecuteScript(ctx, script, arguments, header, snap)
}

// GetAccountAtBlockHeight returns a Flow account by the provided address and block height.
// Expected errors:
// - storage.ErrNotFound if block or register value at height was not found.
func (s *Scripts) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	snap, header, err := s.snapshotWithBlock(height)
	if err != nil {
		return nil, err
	}

	return s.executor.GetAccount(ctx, address, header, snap)
}

// snapshotWithBlock is a common function for executing scripts and get account functionality.
// It creates a storage snapshot that is needed by the FVM to execute scripts.
func (s *Scripts) snapshotWithBlock(height uint64) (snapshot.StorageSnapshot, *flow.Header, error) {
	header, err := s.headers.ByHeight(height)
	if err != nil {
		return nil, nil, err
	}

	storageSnapshot := snapshot.NewReadFuncStorageSnapshot(func(ID flow.RegisterID) (flow.RegisterValue, error) {
		return s.registersAtHeight(ID, height)
	})

	return storageSnapshot, header, nil
}

// IndexRegisterAdapter an adapter for using indexer register values function that takes a slice of IDs in the
// script executor that only uses a single register ID at a time. It also does additional sanity checks if multiple values
// are returned, which shouldn't occur in normal operation.
func IndexRegisterAdapter(registerFun func(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error)) func(flow.RegisterID, uint64) (flow.RegisterValue, error) {
	return func(ID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
		values, err := registerFun([]flow.RegisterID{ID}, height)
		if err != nil {
			return nil, err
		}

		// even though this shouldn't occur in correct implementation we check that function returned either a single register or error
		if len(values) > 1 || len(values) == 0 {
			return nil, fmt.Errorf("invalid number of returned values for a single register: %d", len(values))
		}
		return values[0], nil
	}
}
