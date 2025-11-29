package votecollector

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

var (
	VoteForIncompatibleViewError  = errors.New("vote for incompatible view")
	VoteForIncompatibleBlockError = errors.New("vote for incompatible block")
)

/******************************* NoopProcessor *******************************/

// NoopProcessor implements hotstuff.VoteProcessor. It drops all votes.
type NoopProcessor struct {
	status hotstuff.VoteCollectorStatus
}

func NewNoopCollector(status hotstuff.VoteCollectorStatus) *NoopProcessor {
	return &NoopProcessor{status}
}

func (c *NoopProcessor) Process(*model.Vote) error            { return nil }
func (c *NoopProcessor) Status() hotstuff.VoteCollectorStatus { return c.status }

/************************ enforcing vote is for block ************************/

// EnsureVoteForBlock verifies that the vote is for the given block.
// Returns nil on success and sentinel errors:
//   - [VoteForIncompatibleViewError] if the vote is from a different view than the block
//   - [VoteForIncompatibleBlockError] if the vote is from the same view as the block
//     but for a different blockID
func EnsureVoteForBlock(vote *model.Vote, block *model.Block) error {
	if vote.View != block.View {
		return fmt.Errorf("vote %v has view %d while block's view is %d: %w ", vote.ID(), vote.View, block.View, VoteForIncompatibleViewError)
	}
	if vote.BlockID != block.BlockID {
		return fmt.Errorf("expecting only votes for block %v, but vote %v is for block %v: %w ", block.BlockID, vote.ID(), vote.BlockID, VoteForIncompatibleBlockError)
	}
	return nil
}

// ConcurrentIdentifierSet implements a simple set for tracking unique entries by identifier.
// Removal of set elements is not supported, we want to provide append-only guarantees.
// Concurrency safe.
type ConcurrentIdentifierSet struct {
	set  map[flow.Identifier]struct{}
	lock sync.Mutex
}

// NewConcurrentIdentifierSet creates new identifier set, with strict append-only characteristics.
func NewConcurrentIdentifierSet() *ConcurrentIdentifierSet {
	return &ConcurrentIdentifierSet{
		set: make(map[flow.Identifier]struct{}),
	}
}

// Add adds identifier to the internal set, returns true when added, otherwise returns false.
func (s *ConcurrentIdentifierSet) Add(identifier flow.Identifier) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	_, exists := s.set[identifier]
	if !exists {
		s.set[identifier] = struct{}{}
	}
	return !exists
}
