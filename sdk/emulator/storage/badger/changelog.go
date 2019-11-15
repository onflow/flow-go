package badger

import (
	"sort"
	"sync"
)

// notFound is a sentinel to indicate that a register has never been written by
// a given block height.
const notFound uint64 = ^uint64(0)

// An ordered list of Blocks at which a register changed value. This type
// implements sort.Interface for efficient searching and sorting.
//
// Users should NEVER interact with the backing slice directly, as it must
// be kept sorted for lookups to work.
type changelist struct {
	Blocks []uint64
}

func (c changelist) Len() int { return len(c.Blocks) }

func (c changelist) Less(i, j int) bool { return c.Blocks[i] < c.Blocks[j] }

func (c changelist) Swap(i, j int) { c.Blocks[i], c.Blocks[j] = c.Blocks[j], c.Blocks[i] }

// search finds the highest block number B in the changelist so that B<=n.
// Returns notFound if no such block number exists. This relies on the fact
// that the changelist is kept sorted in ascending order.
func (c changelist) search(n uint64) uint64 {
	if len(c.Blocks) == 0 {
		return notFound
	}
	// This will return the lowest index where the block number is >n.
	// What we want is the index directly BEFORE this.
	foundIndex := sort.Search(c.Len(), func(i int) bool {
		return c.Blocks[i] > n
	})

	if foundIndex == 0 {
		// All block numbers are >n.
		return notFound
	}
	return c.Blocks[foundIndex-1]
}

// add adds the block number to the Blocks, ensuring the Blocks remains sorted. If
// n already exists in the Blocks, this is a no-op.
func (c *changelist) add(n uint64) {
	if n == notFound {
		return
	}

	exists := c.search(n) == n
	if exists {
		return
	}

	c.Blocks = append(c.Blocks, n)
	sort.Sort(c)
}

// The changelog describes the change history of each register in a ledger.
// For each register, the changelog contains a Blocks of all the block numbers at
// which the register's value changed. This enables quick lookups of the latest
// register state change for a given block.
//
// Users of the changelog are responsible for acquiring the mutex before
// reads and writes.
type changelog struct {
	// Maps register IDs to an ordered slice of all the block numbers at which
	// the register value changed.
	registers map[string]changelist
	// Guards the register Blocks from concurrent writes.
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
