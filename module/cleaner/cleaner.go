// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package cleaner

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/storage"
)

// Cleaner is a simple wrapper around our temporary state to clean up after a
// block has been fully finalized to the persistent protocol state.
type Cleaner struct {
	tmp temporary
	per persistent
}

type temporary struct {
	guarantees mempool.Guarantees
	seals      mempool.Seals
}

type persistent struct {
	guarantees storage.Guarantees
	seals      storage.Seals
}

// NewCleaner creates a new cleaner for the temporary state.
func New(guaranteesDB storage.Guarantees, sealsDB storage.Seals, guarantees mempool.Guarantees, seals mempool.Seals) *Cleaner {
	c := &Cleaner{
		tmp: temporary{
			guarantees: guarantees,
			seals:      seals,
		},
		per: persistent{
			guarantees: guaranteesDB,
			seals:      sealsDB,
		},
	}
	return c
}

// CleanAfter will clean up after the block with the given ID has been
// finalized and moved to persistent state with all of its contents.
func (c *Cleaner) CleanAfter(blockID flow.Identifier) error {

	// get the guarantees for the block
	guarantees, err := c.per.guarantees.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not get guarantees from DB: %w", err)
	}

	// get the seals for the block
	seals, err := c.per.seals.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not get seals from DB: %w", err)
	}

	// remove all guarantees from the memory pool
	for _, guarantee := range guarantees {
		ok := c.tmp.guarantees.Rem(guarantee.ID())
		if !ok {
			return fmt.Errorf("could not remove guarantee (collID: %x)", guarantee.ID())
		}
	}

	// remove all seals from the memory pool
	for _, seal := range seals {
		ok := c.tmp.seals.Rem(seal.ID())
		if !ok {
			return fmt.Errorf("could not remove seal (sealID: %x)", seal.ID())
		}
	}

	return nil
}
