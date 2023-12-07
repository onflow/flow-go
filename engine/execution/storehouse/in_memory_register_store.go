package storehouse

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/model/flow"
)

var _ execution.InMemoryRegisterStore = (*InMemoryRegisterStore)(nil)

var ErrNotExecuted = fmt.Errorf("block is not executed")

type PrunedError struct {
	PrunedHeight uint64
	PrunedID     flow.Identifier
	Height       uint64
}

func NewPrunedError(height uint64, prunedHeight uint64, prunedID flow.Identifier) error {
	return PrunedError{Height: height, PrunedHeight: prunedHeight, PrunedID: prunedID}
}

func (e PrunedError) Error() string {
	return fmt.Sprintf("block is pruned at height %d", e.Height)
}

func IsPrunedError(err error) (PrunedError, bool) {
	var e PrunedError
	ok := errors.As(err, &e)
	if ok {
		return e, true
	}
	return PrunedError{}, false
}

type InMemoryRegisterStore struct {
	sync.RWMutex
	registersByBlockID map[flow.Identifier]map[flow.RegisterID]flow.RegisterValue // for storing the registers
	parentByBlockID    map[flow.Identifier]flow.Identifier                        // for register updates to be fork-aware
	blockIDsByHeight   map[uint64]map[flow.Identifier]struct{}                    // for pruning
	prunedHeight       uint64                                                     // registers at pruned height are pruned (not saved in registersByBlockID)
	prunedID           flow.Identifier                                            // to ensure all blocks are extending from pruned block (last finalized and executed block)
}

func NewInMemoryRegisterStore(lastHeight uint64, lastID flow.Identifier) *InMemoryRegisterStore {
	return &InMemoryRegisterStore{
		registersByBlockID: make(map[flow.Identifier]map[flow.RegisterID]flow.RegisterValue),
		parentByBlockID:    make(map[flow.Identifier]flow.Identifier),
		blockIDsByHeight:   make(map[uint64]map[flow.Identifier]struct{}),
		prunedHeight:       lastHeight,
		prunedID:           lastID,
	}
}

// SaveRegisters saves the registers of a block to InMemoryRegisterStore
// It needs to ensure the block is above the pruned height and is connected to the pruned block
func (s *InMemoryRegisterStore) SaveRegisters(
	height uint64,
	blockID flow.Identifier,
	parentID flow.Identifier,
	registers flow.RegisterEntries,
) error {
	// preprocess data before acquiring the lock
	regs := make(map[flow.RegisterID]flow.RegisterValue, len(registers))
	for _, reg := range registers {
		regs[reg.Key] = reg.Value
	}

	s.Lock()
	defer s.Unlock()

	// ensure all saved registers are above the pruned height
	if height <= s.prunedHeight {
		return fmt.Errorf("saving pruned registers height %v <= pruned height %v", height, s.prunedHeight)
	}

	// ensure the block is not already saved
	_, ok := s.registersByBlockID[blockID]
	if ok {
		// already exist
		return fmt.Errorf("saving registers for block %s, but it already exists", blockID)
	}

	// make sure parent is a known block or the pruned block, which forms a fork
	_, ok = s.registersByBlockID[parentID]
	if !ok && parentID != s.prunedID {
		return fmt.Errorf("saving registers for block %s, but its parent %s is not saved", blockID, parentID)
	}

	// update registers for the block
	s.registersByBlockID[blockID] = regs

	// update index on parent
	s.parentByBlockID[blockID] = parentID

	// update index on height
	sameHeight, ok := s.blockIDsByHeight[height]
	if !ok {
		sameHeight = make(map[flow.Identifier]struct{})
		s.blockIDsByHeight[height] = sameHeight
	}

	sameHeight[blockID] = struct{}{}
	return nil
}

// GetRegister will return the latest updated value of the given register
// since the pruned height.
// It returns PrunedError if the register is unknown or not updated since the pruned height
// Can't return ErrNotFound, since we can't distinguish between not found or not updated since the pruned height
func (s *InMemoryRegisterStore) GetRegister(height uint64, blockID flow.Identifier, register flow.RegisterID) (flow.RegisterValue, error) {
	s.RLock()
	defer s.RUnlock()

	if height <= s.prunedHeight {
		return flow.RegisterValue{}, NewPrunedError(height, s.prunedHeight, s.prunedID)
	}

	_, ok := s.registersByBlockID[blockID]
	if !ok {
		return flow.RegisterValue{}, fmt.Errorf("cannot get register at height %d, block %v is not saved: %w", height, blockID, ErrNotExecuted)
	}

	// traverse the fork to find the latest updated value of the given register
	// if not found, it means the register is not updated from the pruned block to the given block
	block := blockID
	for {
		// TODO: do not hold the read lock when reading register from the updated register map
		reg, ok := s.readRegisterAtBlockID(block, register)
		if ok {
			return reg, nil
		}

		// the register didn't get updated at this block, so check its parent

		parent, ok := s.parentByBlockID[block]
		if !ok {
			// if the parent doesn't exist because the block itself is the pruned block,
			// then it means the register is not updated since the pruned height.
			// since we can't distinguish whether the register is not updated or not exist at all,
			// we just return PrunedError error along with the prunedHeight, so the
			// caller could check with OnDiskRegisterStore to find if this register has a updated value
			// at earlier height.
			if block == s.prunedID {
				return flow.RegisterValue{}, NewPrunedError(height, s.prunedHeight, s.prunedID)
			}

			// in this case, it means the state of in-memory register store is inconsistent,
			// because all saved block must have their parent saved in `parentByBlockID`, and traversing
			// its parent should eventually reach the pruned block, otherwise it's a bug.

			return flow.RegisterValue{},
				fmt.Errorf("inconsistent parent block index in in-memory-register-store, ancient block %v is not found when getting register at block %v",
					block, blockID)
		}

		block = parent
	}
}

func (s *InMemoryRegisterStore) readRegisterAtBlockID(blockID flow.Identifier, register flow.RegisterID) (flow.RegisterValue, bool) {
	registers, ok := s.registersByBlockID[blockID]
	if !ok {
		return flow.RegisterValue{}, false
	}

	value, ok := registers[register]
	return value, ok
}

// GetUpdatedRegisters returns the updated registers of a block
func (s *InMemoryRegisterStore) GetUpdatedRegisters(height uint64, blockID flow.Identifier) (flow.RegisterEntries, error) {
	registerUpdates, err := s.getUpdatedRegisters(height, blockID)
	if err != nil {
		return nil, err
	}

	// since the registerUpdates won't be updated and registers for a block can only be set once,
	// we don't need to hold the lock when converting it from map into slice.
	registers := make(flow.RegisterEntries, 0, len(registerUpdates))
	for regID, reg := range registerUpdates {
		registers = append(registers, flow.RegisterEntry{
			Key:   regID,
			Value: reg,
		})
	}

	return registers, nil
}

func (s *InMemoryRegisterStore) getUpdatedRegisters(height uint64, blockID flow.Identifier) (map[flow.RegisterID]flow.RegisterValue, error) {
	s.RLock()
	defer s.RUnlock()
	if height <= s.prunedHeight {
		return nil, fmt.Errorf("cannot get register at height %d, it is pruned %v", height, s.prunedHeight)
	}

	registerUpdates, ok := s.registersByBlockID[blockID]
	if !ok {
		return nil, fmt.Errorf("cannot get register at height %d, block %s is not found: %w", height, blockID, ErrNotExecuted)
	}
	return registerUpdates, nil
}

// Prune prunes the register store to the given height
// The pruned height must be an executed block, the caller should ensure that by calling SaveRegisters before.
//
// Pruning is done by walking up the finalized fork from `s.prunedHeight` to `height`. At each height, prune all
// other forks that begin at that height. This ensures that data for all conflicting forks are freed
//
// TODO: It does not block the caller, the pruning work is done async
func (s *InMemoryRegisterStore) Prune(height uint64, blockID flow.Identifier) error {
	finalizedFork, err := s.findFinalizedFork(height, blockID)
	if err != nil {
		return fmt.Errorf("cannot find finalized fork: %w", err)
	}

	s.Lock()
	defer s.Unlock()

	// prune each height starting at the lowest height in the fork. this will remove all blocks
	// below the new pruned height along with any conflicting forks.
	for i := len(finalizedFork) - 1; i >= 0; i-- {
		blockID := finalizedFork[i]

		err := s.pruneByHeight(s.prunedHeight+1, blockID)
		if err != nil {
			return fmt.Errorf("could not prune by height %v: %w", s.prunedHeight+1, err)
		}
	}

	return nil
}

func (s *InMemoryRegisterStore) PrunedHeight() uint64 {
	s.RLock()
	defer s.RUnlock()
	return s.prunedHeight
}

func (s *InMemoryRegisterStore) IsBlockExecuted(height uint64, blockID flow.Identifier) (bool, error) {
	s.RLock()
	defer s.RUnlock()

	// finalized and executed blocks are pruned
	if height < s.prunedHeight {
		return false, fmt.Errorf("below pruned height")
	}

	if height == s.prunedHeight {
		return blockID == s.prunedID, nil
	}

	_, ok := s.registersByBlockID[blockID]
	return ok, nil
}

// findFinalizedFork returns the finalized fork from higher height to lower height
// the last block's height is s.prunedHeight + 1
func (s *InMemoryRegisterStore) findFinalizedFork(height uint64, blockID flow.Identifier) ([]flow.Identifier, error) {
	s.RLock()
	defer s.RUnlock()

	if height < s.prunedHeight {
		return nil, fmt.Errorf("cannot find finalized fork at height %d, it is pruned (prunedHeight: %v)", height, s.prunedHeight)
	}

	if height == s.prunedHeight {
		if blockID != s.prunedID {
			return nil, fmt.Errorf("cannot find finalized fork at height %d, it is pruned (prunedHeight: %v, prunedID: %v)", height, s.prunedHeight, s.prunedID)
		}

		return nil, nil
	}

	prunedHeight := height
	block := blockID

	// walk backwards from the provided finalized block to the last pruned block
	// the result must be a chain from height/blockID to s.prunedHeight/s.prunedID
	fork := make([]flow.Identifier, 0, height-s.prunedHeight)
	for {
		fork = append(fork, block)
		prunedHeight--

		parent, ok := s.parentByBlockID[block]
		if !ok {
			return nil, fmt.Errorf("inconsistent parent block index in in-memory-register-store, ancient block %s is not found when finding finalized fork at height %v", block, height)
		}
		if parent == s.prunedID {
			break
		}
		block = parent
	}

	if prunedHeight != s.prunedHeight {
		return nil, fmt.Errorf("inconsistent parent block index in in-memory-register-store, pruned height %d is not equal to %d", prunedHeight, s.prunedHeight)
	}

	return fork, nil
}

func (s *InMemoryRegisterStore) pruneByHeight(height uint64, finalized flow.Identifier) error {
	s.removeBlock(height, finalized)

	// remove conflicting forks
	for blockID := range s.blockIDsByHeight[height] {
		s.pruneFork(height, blockID)
	}

	if len(s.blockIDsByHeight[height]) > 0 {
		return fmt.Errorf("all forks on the same height should have been pruend, but actually not: %v", len(s.blockIDsByHeight[height]))
	}

	delete(s.blockIDsByHeight, height)
	s.prunedHeight = height
	s.prunedID = finalized
	return nil
}

func (s *InMemoryRegisterStore) removeBlock(height uint64, blockID flow.Identifier) {
	delete(s.registersByBlockID, blockID)
	delete(s.parentByBlockID, blockID)
	delete(s.blockIDsByHeight[height], blockID)
}

// pruneFork prunes the provided block and all of its children
func (s *InMemoryRegisterStore) pruneFork(height uint64, blockID flow.Identifier) {
	s.removeBlock(height, blockID)
	// all its children must be at height + 1, whose parent is blockID

	nextHeight := height + 1
	blocksAtNextHeight, ok := s.blockIDsByHeight[nextHeight]
	if !ok {
		return
	}

	for block := range blocksAtNextHeight {
		isChild := s.parentByBlockID[block] == blockID
		if isChild {
			s.pruneFork(nextHeight, block)
		}
	}
}
