package badger

import (
	"sort"
	"sync"
)

// notFound is a sentinel to indicate that a register has never been written by
// a given block height.
const notFound uint64 = ^uint64(0)

// An ordered list of blocks at which a register changed value. This type
// implements sort.Interface for efficient searching and sorting.
//
// Users should NEVER interact with the backing slice directly, as it must
// be kept sorted for lookups to work.
type changelist struct {
	blocks []uint64
}

// Implement sort.Interface for testing sortedness.
func (c changelist) Len() int { return len(c.blocks) }

func (c changelist) Less(i, j int) bool { return c.blocks[i] < c.blocks[j] }

func (c changelist) Swap(i, j int) { c.blocks[i], c.blocks[j] = c.blocks[j], c.blocks[i] }

// search finds the highest block number B in the changelist so that B<=n.
// Returns notFound if no such block number exists. This relies on the fact
// that the changelist is kept sorted in ascending order.
func (c changelist) search(n uint64) uint64 {
	index := c.searchForIndex(n)
	if index == -1 {
		return notFound
	}
	return c.blocks[index]
}

// searchForIndex finds the index of the highest block number B in the
// changelist so that B<=n. Returns -1 if no such block number exists. This
// relies on the fact that the changelist is kept sorted in ascending order.
func (c changelist) searchForIndex(n uint64) (index int) {
	if len(c.blocks) == 0 {
		return -1
	}
	// This will return the lowest index where the block number is >n.
	// What we want is the index directly BEFORE this.
	foundIndex := sort.Search(c.Len(), func(i int) bool {
		return c.blocks[i] > n
	})

	if foundIndex == 0 {
		// All block numbers are >n.
		return -1
	}
	return foundIndex - 1
}

// add adds the block number to the list, ensuring the list remains sorted. If
// n already exists in the list, this is a no-op.
func (c *changelist) add(n uint64) {
	if n == notFound {
		return
	}

	index := c.searchForIndex(n)
	if index == -1 {
		// all blocks in the list are >n, or the list is empty
		c.blocks = append([]uint64{n}, c.blocks...)
		return
	}

	lastBlockNumber := c.blocks[index]
	if lastBlockNumber == n {
		// n already exists in the list
		return
	}

	// insert n directly after lastBlockNumber
	c.blocks = append(c.blocks[:index+1], append([]uint64{n}, c.blocks[index+1:]...)...)
}

// The changelog describes the change history of each register in a ledger.
// For each register, the changelog contains a list of all the block numbers at
// which the register's value changed. This enables quick lookups of the latest
// register state change for a given block.
//
// Users of the changelog are responsible for acquiring the mutex before
// reads and writes.
type changelog struct {
	// Maps register IDs to an ordered slice of all the block numbers at which
	// the register value changed.
	registers map[string]changelist
	// Guards the register list from concurrent writes.
	sync.RWMutex
}

// newChangelog returns a new changelog.
func newChangelog() changelog {
	return changelog{
		registers: make(map[string]changelist),
		RWMutex:   sync.RWMutex{},
	}
}

// getMostRecentChange returns the most recent block number at which the
// register with the given ID changed value.
func (c changelog) getMostRecentChange(registerID string, blockNumber uint64) uint64 {
	clist, ok := c.registers[registerID]
	if !ok {
		return notFound
	}

	return clist.search(blockNumber)
}

// changelists returns an exhaustive list of changelists keyed by register ID.
func (c changelog) changelists() map[string]changelist {
	return c.registers
}

// getChangelist returns the changelist corresponding to the given register ID.
// Returns an empty changelist if none exists.
func (c changelog) getChangelist(registerID string) changelist {
	return c.registers[registerID]
}

// setChangelist sets the changelist for the given register ID, discarding the
// existing changelist if one exists.
func (c changelog) setChangelist(registerID string, clist changelist) {
	c.registers[registerID] = clist
}

// addChange adds a change record to the given register at the given block.
// If the changelist already reflects a change for this register at this block,
// this is a no-op.
//
// If the changelist doesn't exist, it is created.
func (c *changelog) addChange(registerID string, blockNumber uint64) {
	clist := c.registers[registerID]
	clist.add(blockNumber)
	c.registers[registerID] = clist
}
