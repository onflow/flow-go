package recovery

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// BlockScanner describes a function for ingesting pending blocks.
// Any returned errors are considered fatal.
type BlockScanner func(proposal *model.Proposal) error

// Recover is a utility method for recovering the HotStuff state after a restart.
// It receives the list `pending` containing _all_ blocks that
//   - have passed the compliance layer and stored in the protocol state
//   - descend from the latest finalized block
//   - are listed in ancestor-first order (i.e. for any block B ∈ pending, B's parent must
//     be listed before B, unless B's parent is the latest finalized block)
//
// CAUTION: all pending blocks are required to be valid (guaranteed if the block passed the compliance layer)
func Recover(log zerolog.Logger, pending []*flow.Header, scanners ...BlockScanner) error {
	log.Info().Int("total", len(pending)).Msgf("recovery started")

	// add all pending blocks to forks
	for _, header := range pending {
		proposal := model.ProposalFromFlow(header) // convert the header into a proposal
		for _, s := range scanners {
			err := s(proposal)
			if err != nil {
				return fmt.Errorf("scanner failed to ingest proposal: %w", err)
			}
		}
		log.Debug().
			Uint64("view", proposal.Block.View).
			Hex("block_id", proposal.Block.BlockID[:]).
			Msg("block recovered")
	}

	log.Info().Msgf("recovery completed")
	return nil
}

// ForksState recovers Forks' internal state of blocks descending from the latest
// finalized block. Caution, input blocks must be valid and in parent-first order
// (unless parent is the latest finalized block).
func ForksState(forks hotstuff.Forks) BlockScanner {
	return func(proposal *model.Proposal) error {
		err := forks.AddValidatedBlock(proposal.Block)
		if err != nil {
			return fmt.Errorf("could not add block %v to forks: %w", proposal.Block.BlockID, err)
		}
		return nil
	}
}

// VoteAggregatorState recovers the VoteAggregator's internal state as follows:
//   - Add all blocks descending from the latest finalized block to accept votes.
//     Those blocks should be rapidly pruned as the node catches up.
//
// Caution: input blocks must be valid.
func VoteAggregatorState(voteAggregator hotstuff.VoteAggregator) BlockScanner {
	return func(proposal *model.Proposal) error {
		voteAggregator.AddBlock(proposal)
		return nil
	}
}

// CollectParentQCs collects all parent QCs included in the blocks descending from the
// latest finalized block. Caution, input blocks must be valid.
func CollectParentQCs(collector Collector[*flow.QuorumCertificate]) BlockScanner {
	return func(proposal *model.Proposal) error {
		qc := proposal.Block.QC
		if qc != nil {
			collector.Append(qc)
		}
		return nil
	}
}

// CollectTCs collect all TCs included in the blocks descending from the
// latest finalized block. Caution, input blocks must be valid.
func CollectTCs(collector Collector[*flow.TimeoutCertificate]) BlockScanner {
	return func(proposal *model.Proposal) error {
		tc := proposal.LastViewTC
		if tc != nil {
			collector.Append(tc)
		}
		return nil
	}
}

// Collector for objects of generic type. Essentially, it is a stateful list.
// Safe to be passed by value. Retrieve() returns the current state of the list
// and is unaffected by subsequent appends.
type Collector[T any] struct {
	list *[]T
}

func NewCollector[T any]() Collector[T] {
	list := make([]T, 0, 5) // heuristic: pre-allocate with some basic capacity
	return Collector[T]{list: &list}
}

// Append adds new elements to the end of the list.
func (c Collector[T]) Append(t ...T) {
	*c.list = append(*c.list, t...)
}

// Retrieve returns the current state of the list (unaffected by subsequent append)
func (c Collector[T]) Retrieve() []T {
	// Under the hood, the slice is a struct containing a pointer to an underlying array and a
	// `len` variable indicating how many of the array elements are occupied. Here, we are
	// returning the slice struct by value, i.e. we _copy_ the array pointer and the `len` value
	// and return the copy. Therefore, the returned slice is unaffected by subsequent append.
	return *c.list
}
