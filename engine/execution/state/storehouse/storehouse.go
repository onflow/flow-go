package storehouse

import (
	"sync"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

type StoreHouse struct {
	lock                 sync.Mutex
	forkStore            state.ForkAwareStore
	database             state.NonForkAwareStorage
	getFinalizedByHeight func(height uint64) (*flow.Header, bool, error)
}

type HousedSnapshot struct {
	view      uint64
	blockID   flow.Identifier
	forkStore state.ForkAwareStore
	database  state.NonForkAwareStorage
}

func (s *StoreHouse) BlockView(view uint64, blockID flow.Identifier) snapshot.StorageSnapshot {
	return &HousedSnapshot{
		view:      view,
		blockID:   blockID,
		forkStore: s.forkStore,
		database:  s.database,
	}
}

func isUnexected(err error) bool {
	return false
}

func (s *HousedSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	// why lookup from the fork store first?
	// because fork store and database have overlap
	value, err := s.forkStore.GetRegsiter(s.view, id)
	if err == nil {
		return value, nil
	}

	if isUnexected(err) {
		return nil, err // should not happen, the caller should ensure the block must have been executed
	}

	// value not found, try in storage
	return s.database.GetRegsiter(s.view, id)
}

func (s *StoreHouse) StoreBlock(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// lastCommit is a finalized and executed block
	lastCommit, err := s.database.LastCommitBlock()
	if err != nil {
		return err
	}

	// we must have that block in the database, ignore it
	if header.Height <= lastCommit.Height {
		return nil
	}

	finalized, hasFinalized, err := s.getFinalizedByHeight(lastCommit.Height + 1)
	if err != nil {
		return err
	}

	if !hasFinalized {
		return s.forkStore.AddForBlock(header, update)
	}

	if header.ID() != finalized.ID() {
		// executed a non-finalized block, through away
		return nil
	}

	// execution falling behind, we just executed a finalized block
	err = s.database.CommitBlock(header, update)
	if err != nil {
		return err
	}

	return s.forkStore.PruneByFinalized(finalized)
}

// BlockFinalized notify the StoreHouse about block finalization, so that StoreHouse can
// commit the register updates for the block and move them into non-forkaware storage
func (s *StoreHouse) BlockFinalized(finalized *flow.Header) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	lastCommit, err := s.database.LastCommitBlock()
	if err != nil {
		return err
	}

	nextToCommit := lastCommit.Height + 1

	if finalized.Height > nextToCommit {
		// finalization is ahead of execution, will ignore the finalization event
		return nil
	}

	if finalized.Height < nextToCommit {
		// the finalized block has already stored in database, should not happen
		return nil
	}

	// finalized.Height == nextToCommit, finalized is the next block to commit, then
	// check if it has been executed, if yes, then commit it
	updates, err := s.forkStore.GetUpdatesByBlock(finalized.ID()) // or using view
	if err != nil {
		// this means not found
		// the block has not been executed yet, ignore
		return nil
	}

	err = s.database.CommitBlock(finalized, updates)
	if err != nil {
		return err
	}

	return s.forkStore.PruneByFinalized(finalized)
}
