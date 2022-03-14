// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package protocol

import (
	"github.com/onflow/flow-go/model/flow"
)

// Snapshot represents an immutable snapshot of the protocol state
// at a specific block, denoted as the Head block.
// The Snapshot is fork-specific and only accounts for the information contained
// in blocks along this fork up to (including) Head.
// It allows us to read the parameters at the selected block in a deterministic manner.
type Snapshot interface {

	// Head returns the latest block at the selected point of the protocol state
	// history. It can represent either a finalized or ambiguous block,
	// depending on our selection criteria. Either way, it's the block on which
	// we should build the next block in the context of the selected state.
	Head() (*flow.Header, error)

	// QuorumCertificate returns a valid quorum certificate for the header at
	// this snapshot, if one exists.
	QuorumCertificate() (*flow.QuorumCertificate, error)

	// Identities returns a list of identities at the selected point of the
	// protocol state history. At the beginning of an epoch, this list includes
	// identities from the previous epoch that are un-staking during the current
	// epoch. At the end of an epoch, this includes identities scheduled to join
	// in the next epoch but are not active yet.
	//
	// Identities are guaranteed to be returned in canonical order (order.Canonical).
	//
	// It allows us to provide optional upfront filters which can be used by the
	// implementation to speed up database lookups.
	Identities(selector flow.IdentityFilter) (flow.IdentityList, error)

	// Identity attempts to retrieve the node with the given identifier at the
	// selected point of the protocol state history. It will error if it doesn't exist.
	Identity(nodeID flow.Identifier) (*flow.Identity, error)

	// SealedResult returns the most recent included seal as of this block and
	// the corresponding execution result. The seal may have been included in a
	// parent block, if this block is empty. If this block contains multiple
	// seals, this returns the seal for the block with the greatest height.
	SealedResult() (*flow.ExecutionResult, *flow.Seal, error)

	// Commit returns the state commitment of the most recently included seal
	// as of this block. It represents the sealed state.
	Commit() (flow.StateCommitment, error)

	// SealingSegment returns the chain segment such that the highest block
	// is this snapshot's reference block and the lowest block
	// is the most recently sealed block as of this snapshot (ie. the block
	// referenced by LatestSeal). The segment is in ascending height order.
	//
	// TAIL <- B1 <- ... <- BN <- HEAD
	//
	// NOTE 1: TAIL is not always sealed by HEAD. In the case that the head of
	// the snapshot contains no seals, TAIL must be sealed by the first ancestor
	// of HEAD which contains any seal.
	//
	// NOTE 2: In the special case of a root snapshot generated for a spork,
	// the sealing segment has exactly one block (the root block for the spork).
	// For all other snapshots, the sealing segment contains at least 2 blocks.
	//
	// NOTE 3: It is often the case that a block inside the segment will contain
	// an execution receipt in it's payload that references an execution result
	// missing from the payload. These missing execution results are stored on the
	// flow.SealingSegment.ExecutionResults field.
	SealingSegment() (*flow.SealingSegment, error)

	// Descendants returns the IDs of all descendants of the Head block. The IDs
	// are ordered such that parents are included before their children. These
	// are NOT guaranteed to have been validated by HotStuff.
	Descendants() ([]flow.Identifier, error)

	// ValidDescendants returns the IDs of all descendants of the Head block.
	// The IDs are ordered such that parents are included before their children. Includes
	// only blocks that have been validated by HotStuff.
	ValidDescendants() ([]flow.Identifier, error)

	// RandomSource returns the source of randomness derived from the
	// Head block.
	// NOTE: not to be confused with the epoch source of randomness!
	// error returns:
	//  * NoValidChildBlockError indicates that no valid child block is known
	//    (which contains the block's source of randomness)
	//  * unexpected errors should be considered symptoms of internal bugs
	RandomSource() ([]byte, error)

	// Phase returns the epoch phase for the current epoch, as of the Head block.
	Phase() (flow.EpochPhase, error)

	// Epochs returns a query object enabling querying detailed information about
	// various epochs.
	//
	// For epochs that are in the future w.r.t. the Head block, some of Epoch's
	// methods may return errors, since the Epoch Preparation Protocol may be
	// in-progress and incomplete for the epoch.
	Epochs() EpochQuery

	// Params returns global parameters of the state this snapshot is taken from.
	Params() GlobalParams
}
