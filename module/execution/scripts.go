package execution

import (
	"context"

	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/storage"
)

type Scripts struct {
	executor query.QueryExecutor
	headers  storage.Headers
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

	return s.executor.ExecuteScript(ctx, script, arguments, header, snapshot)
}
