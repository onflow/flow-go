package state_synchronization

import "github.com/onflow/flow-go/model/flow"

type Storage struct {
}

func (s *Storage) Queue(rootID flow.Identifier) {

}

func (s *Storage) StartTransfer(rootID flow.Identifier) {

}

func (s *Storage) FinishTransfer(rootID flow.Identifier) {

}

// TODO: we need to add the refcount before we start the request
// This prevents the race condition of requesting a block, then it being deleted,
// then we update the refcount

// what happens on the very first request?
// maybe we can define a datastore that knows 

func (s *Storage) ReceivedRoot(rootID flow.Identifier, chunkExecutionDataIDs []flow.Identifier) {

}

func (s *Storage) ReceivedLevel(parentLevel, level []cid.Cid){

}

func (s *Storage) 
