package storehouse

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type RegisterStore struct {
	memStore  *InMemoryRegisterStore
	diskStore execution.OnDiskRegisterStore
	wal       execution.ExecutedFinalizedWAL
	finalized execution.FinalizedReader
	log       zerolog.Logger
}

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
	finalizedID, err := finalized.GetFinalizedBlockIDAtHeight(height)
	if err != nil {
		return nil, fmt.Errorf("cannot get finalized block ID at height %d: %w", height, err)
	}

	// init the memStore with the last executed and finalized block ID
	memStore := NewInMemoryRegisterStore(height, finalizedID)

	return &RegisterStore{
		memStore:  memStore,
		diskStore: diskStore,
		wal:       wal,
		finalized: finalized,
		log:       log.With().Str("module", "register-store").Logger(),
	}, nil
}

// GetRegister first try to get the register from InMemoryRegisterStore, then OnDiskRegisterStore
func (r *RegisterStore) GetRegister(height uint64, blockID flow.Identifier, register flow.RegisterID) (flow.RegisterValue, error) {
	reg, err := r.memStore.GetRegister(height, blockID, register)
	// the height might be lower than the lowest height in memStore,
	// or the register might not be found in memStore,
	// in both cases, we need to get the register from diskStore
	// there is no other error type
	if err != nil {
		return r.diskStore.Get(register, height)
	}
	return reg, nil
}

// SaveRegisters saves to InMemoryRegisterStore first, then trigger the same check as OnBlockFinalized
// Depend on InMemoryRegisterStore.SaveRegisters
func (r *RegisterStore) SaveRegisters(header *flow.Header, registers []flow.RegisterEntry) error {
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

// Depend on FinalizedReader's GetFinalizedBlockIDAtHeight
// Depend on ExecutedFinalizedWAL.Append
// Depend on OnDiskRegisterStore.SaveRegisters
// OnBlockFinalized trigger the check of whether a block at the next height becomes finalized and executed.
// the next height is the existing finalized and executed block's height + 1.
// If a block at next height becomes finalized and executed, then:
// 1. write the registers to write ahead logs
// 2. save the registers of the block to OnDiskRegisterStore
// 3. prune the height in InMemoryRegisterStore
func (r *RegisterStore) OnBlockFinalized() error {
	latest := r.diskStore.LatestHeight()
	next := latest + 1
	blockID, err := r.finalized.GetFinalizedBlockIDAtHeight(next)
	if errors.Is(err, storage.ErrNotFound) {
		// next block is not finalized yet
		return nil
	}

	regs, err := r.memStore.GetUpdatedRegisters(next, blockID)
	// TODO: use other error type
	if errors.Is(err, storage.ErrNotFound) {
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

	return nil
}

// FinalizedAndExecutedHeight returns the height of the last finalized and executed block,
// which has been saved in OnDiskRegisterStore
func (r *RegisterStore) FinalizedAndExecutedHeight() uint64 {
	// diskStore caches the latest height in memory
	return r.diskStore.LatestHeight()
}

// IsBlockExecuted returns true if the block is executed, false if not executed
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
	finalizedID, err := r.finalized.GetFinalizedBlockIDAtHeight(height)
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
