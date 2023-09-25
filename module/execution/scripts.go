package execution

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// RegistersAtHeight returns register values for provided register IDs at the block height.
// Even if the register wasn't indexed at the provided height, returns the highest height the register was indexed at.
// Expected errors:
// - storage.ErrNotFound if the register by the ID was never indexed
// - ErrIndexBoundary if the height is out of indexed height boundary
type RegistersAtHeight func(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error)

type Scripts struct {
	executor          query.QueryExecutor
	headers           storage.Headers
	registersAtHeight RegistersAtHeight
}

func NewScripts(executor query.QueryExecutor, header storage.Headers, registersAtHeight RegistersAtHeight) *Scripts {
	return &Scripts{
		executor:          executor,
		headers:           header,
		registersAtHeight: registersAtHeight,
	}
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
