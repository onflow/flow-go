package state_requester

import (
	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/engine/execution/state_synchronization"
)

type ExecutionDataRequest interface {
	ExecutionData() *state_synchronization.ExecutionData
	CID() cid.Cid
	Error() error
	Done() <-chan struct{}
}

type executionDataRequestImpl struct {
	executionData *state_synchronization.ExecutionData
	cid           cid.Cid
	err           error
	done          chan struct{}
}

func (r *executionDataRequestImpl) ExecutionData() *state_synchronization.ExecutionData {
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
