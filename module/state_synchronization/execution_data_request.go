package state_synchronization

import (
	"github.com/ipfs/go-cid"
)

type ExecutionDataRequest interface {
	ExecutionData() *ExecutionData
	CID() cid.Cid
	Error() error
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
