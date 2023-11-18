package storehouse

import (
	"errors"
	"fmt"

	"go.uber.org/atomic"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type RegisterStore struct {
	memStore   *InMemoryRegisterStore
	diskStore  execution.OnDiskRegisterStore
	wal        execution.ExecutedFinalizedWAL
	finalized  execution.FinalizedReader
	log        zerolog.Logger
	finalizing *atomic.Bool // making sure only one goroutine is finalizing at a time
}

var _ execution.RegisterStore = (*RegisterStore)(nil)

func NewRegisterStore(
	diskStore execution.OnDiskRegisterStore,
	wal execution.ExecutedFinalizedWAL,
	finalized execution.FinalizedReader,
	log zerolog.Logger,
) (*RegisterStore, error) {
	// replay the executed and finalized blocks from the write ahead logs
	// to the OnDiskRegisterStore
	height, err := syncDiskStore(wal, diskStore, log)
	if err != nil {
		return nil, fmt.Errorf("cannot sync disk store: %w", err)
	}

	// fetch the last executed and finalized block ID
	finalizedID, err := finalized.FinalizedBlockIDAtHeight(height)
	if err != nil {
		return nil, fmt.Errorf("cannot get finalized block ID at height %d: %w", height, err)
	}

	// init the memStore with the last executed and finalized block ID
	memStore := NewInMemoryRegisterStore(height, finalizedID)

	return &RegisterStore{
		memStore:   memStore,
		diskStore:  diskStore,
		wal:        wal,
		finalized:  finalized,
		finalizing: atomic.NewBool(false),
		log:        log.With().Str("module", "register-store").Logger(),
	}, nil
}

// GetRegister first try to get the register from InMemoryRegisterStore, then OnDiskRegisterStore
// 1. below pruned height, and is conflicting
// 2. below pruned height, and is finalized
// 3. above pruned height, and is not executed
// 4. above pruned height, and is executed, and register is updated
// 5. above pruned height, and is executed, but register is not updated since pruned height
// It returns:
//   - (value, nil) if the register value is found at the given block
//   - (nil, nil) if the register is not found
//   - (nil, storage.ErrHeightNotIndexed) if the height is below the first height that is indexed.
//   - (nil, storehouse.ErrNotExecuted) if the block is not executed yet
//   - (nil, storehouse.ErrNotExecuted) if the block is conflicting iwth finalized block
//   - (nil, err) for any other exceptions
func (r *RegisterStore) GetRegister(height uint64, blockID flow.Identifier, register flow.RegisterID) (flow.RegisterValue, error) {
	reg, err := r.memStore.GetRegister(height, blockID, register)
	// the height might be lower than the lowest height in memStore,
	// or the register might not be found in memStore.
	if err == nil {
		// this register was updated before its block is finalized
		return reg, nil
	}

	prunedError, ok := IsPrunedError(err)
	if !ok {
		// this means we ran into an exception. finding a register from in-memory store should either
		// getting the register value or getting a ErrPruned error.
		return flow.RegisterValue{}, fmt.Errorf("cannot get register from memStore: %w", err)
	}

	// if in memory store returns PrunedError, and register height is above the pruned height,
	// then it means the block is connected to the pruned block of in memory store, which is
	// a finalized block and executed block, so we can get its value from on disk store.
	if height > prunedError.PrunedHeight {
		return r.getAndConvertNotFoundErr(register, prunedError.PrunedHeight)
	}

	// if the block is below the pruned height, then there are two cases:
	// the block is a finalized block, or a conflicting block.
	// In order to distinguish, we need to query the finalized block ID at that height
	finalizedID, err := r.finalized.FinalizedBlockIDAtHeight(height)
	if err != nil {
		return nil, fmt.Errorf("cannot get finalized block ID at height %d: %w", height, err)
	}

	isConflictingBlock := blockID != finalizedID
	if isConflictingBlock {
		// conflicting blocks are considered as un-executed
		return flow.RegisterValue{}, fmt.Errorf("getting registers from conflicting block %v at height %v: %w", blockID, height, ErrNotExecuted)
	}
	return r.getAndConvertNotFoundErr(register, height)
}

// getAndConvertNotFoundErr returns nil if the register is not found from storage
func (r *RegisterStore) getAndConvertNotFoundErr(register flow.RegisterID, height uint64) (flow.RegisterValue, error) {
	val, err := r.diskStore.Get(register, height)
	if errors.Is(err, storage.ErrNotFound) {
		// FVM expects the error to be nil when register is not found
		return nil, nil
	}
	return val, err
}

// SaveRegisters saves to InMemoryRegisterStore first, then trigger the same check as OnBlockFinalized
// Depend on InMemoryRegisterStore.SaveRegisters
// It returns:
// - nil if the registers are saved successfully
// - exception is the block is above the pruned height but does not connect to the pruned height (conflicting block).
// - exception if the block is below the pruned height
// - exception if the save block is saved again
// - exception for any other exception
func (r *RegisterStore) SaveRegisters(header *flow.Header, registers flow.RegisterEntries) error {
	err := r.memStore.SaveRegisters(header.Height, header.ID(), header.ParentID, registers)
	if err != nil {
		return fmt.Errorf("cannot save register to memStore: %w", err)
	}

	err = r.OnBlockFinalized()
	if err != nil {
		return fmt.Errorf("cannot trigger OnBlockFinalized: %w", err)
	}
	return nil
}

// Depend on FinalizedReader's FinalizedBlockIDAtHeight
// Depend on ExecutedFinalizedWAL.Append
// Depend on OnDiskRegisterStore.SaveRegisters
// OnBlockFinalized trigger the check of whether a block at the next height becomes finalized and executed.
// the next height is the existing finalized and executed block's height + 1.
// If a block at next height becomes finalized and executed, then:
// 1. write the registers to write ahead logs
// 2. save the registers of the block to OnDiskRegisterStore
// 3. prune the height in InMemoryRegisterStore
func (r *RegisterStore) OnBlockFinalized() error {
	// only one goroutine can execute OnBlockFinalized at a time
	if !r.finalizing.CompareAndSwap(false, true) {
		return nil
	}

	defer r.finalizing.Store(false)
	return r.onBlockFinalized()
}

func (r *RegisterStore) onBlockFinalized() error {
	latest := r.diskStore.LatestHeight()
	next := latest + 1
	blockID, err := r.finalized.FinalizedBlockIDAtHeight(next)
	if errors.Is(err, storage.ErrNotFound) {
		// next block is not finalized yet
		return nil
	}

	regs, err := r.memStore.GetUpdatedRegisters(next, blockID)
	if errors.Is(err, ErrNotExecuted) {
		// next block is not executed yet
		return nil
	}

	// TODO: append WAL
	// err = r.wal.Append(next, regs)
	// if err != nil {
	// 	return fmt.Errorf("cannot write %v registers to write ahead logs for height %v: %w", len(regs), next, err)
	// }

	err = r.diskStore.Store(regs, next)
	if err != nil {
		return fmt.Errorf("cannot save %v registers to disk store for height %v: %w", len(regs), next, err)
	}

	err = r.memStore.Prune(next, blockID)
	if err != nil {
		return fmt.Errorf("cannot prune memStore for height %v: %w", next, err)
	}

	return r.onBlockFinalized() // check again until there is no more finalized block
}

// LastFinalizedAndExecutedHeight returns the height of the last finalized and executed block,
// which has been saved in OnDiskRegisterStore
func (r *RegisterStore) LastFinalizedAndExecutedHeight() uint64 {
	// diskStore caches the latest height in memory
	return r.diskStore.LatestHeight()
}

// IsBlockExecuted returns true if the block is executed, false if not executed
// Note: it returns (true, nil) even if the block has been pruned from on disk register store,
func (r *RegisterStore) IsBlockExecuted(height uint64, blockID flow.Identifier) (bool, error) {
	executed, err := r.memStore.IsBlockExecuted(height, blockID)
	if err != nil {
		// the only error memStore would return is when the given height is lower than the pruned height in memStore.
		// Since the pruned height in memStore is a finalized and executed height, in order to know if the block
		// is executed, we just need to check if this block is the finalized blcok at the given height.
		executed, err = r.isBlockFinalized(height, blockID)
		return executed, err
	}

	return executed, nil
}

func (r *RegisterStore) isBlockFinalized(height uint64, blockID flow.Identifier) (bool, error) {
	finalizedID, err := r.finalized.FinalizedBlockIDAtHeight(height)
	if err != nil {
		return false, fmt.Errorf("cannot get finalized block ID at height %d: %w", height, err)
	}
	return finalizedID == blockID, nil
}

// syncDiskStore replay WAL to disk store
func syncDiskStore(
	wal execution.ExecutedFinalizedWAL,
	diskStore execution.OnDiskRegisterStore,
	log zerolog.Logger,
) (uint64, error) {
	// TODO: replace diskStore.Latest with wal.Latest
	// latest, err := r.wal.Latest()
	var err error
	latest := diskStore.LatestHeight() // tmp
	if err != nil {
		return 0, fmt.Errorf("cannot get latest height from write ahead logs: %w", err)
	}

	stored := diskStore.LatestHeight()

	if stored > latest {
		return 0, fmt.Errorf("latest height in storehouse %v is larger than latest height %v in write ahead logs", stored, latest)
	}

	if stored < latest {
		// replay
		reader := wal.GetReader(stored + 1)
		for {
			height, registers, err := reader.Next()
			// TODO: to rename
			if errors.Is(err, storage.ErrNotFound) {
				break
			}
			if err != nil {
				return 0, fmt.Errorf("cannot read registers from write ahead logs: %w", err)
			}

			err = diskStore.Store(registers, height)
			if err != nil {
				return 0, fmt.Errorf("cannot save registers to disk store at height %v : %w", height, err)
			}
		}
	}

	return latest, nil
}
