package state_synchronization

import (
	"github.com/ipfs/go-cid"
)

// ExecutionDataRequest represents a request to get the execution data of a block
type ExecutionDataRequest interface {
	// If Done is not yet closed, ExecutionData returns nil. If Done is closed and the request was successful,
	// ExecutionData returns the execution data of the block.
	ExecutionData() *ExecutionData

	// CID returns the root CID of the execution data being requested.
	CID() cid.Cid

	// If Done is not yet closed, Error returns nil. If Done is closed and the request was unsuccessful,
	// Error returns the error.
	Error() error

	// Done returns a channel that is closed when the request is completed
	Done() <-chan struct{}
}

type executionDataRequestImpl struct {
	executionData *ExecutionData
	cid           cid.Cid
	err           error
	done          chan struct{}
}

func (r *executionDataRequestImpl) ExecutionData() *ExecutionData {
	return r.executionData
}

func (r *executionDataRequestImpl) CID() cid.Cid {
	return r.cid
}

func (r *executionDataRequestImpl) Error() error {
	return r.err
}

func (r *executionDataRequestImpl) Done() <-chan struct{} {
	return r.done
}

func (r *executionDataRequestImpl) setExecutionData(e *ExecutionData) {
	r.executionData = e
	close(r.done)
}

func (r *executionDataRequestImpl) setError(err error) {
	r.err = err
	close(r.done)
}
