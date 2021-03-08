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

	// Identities returns a list of identities at the selected point of the
	// protocol state history. At the beginning of an epoch, this list includes
	// identities from the previous epoch that are un-staking during the current
	// epoch. At the end of an epoch, this includes identities scheduled to join
	// in the next epoch but are not active yet.
	//
	// It allows us to provide optional upfront filters which can be used by the
	// implementation to speed up database lookups.
	Identities(selector flow.IdentityFilter) (flow.IdentityList, error)

	// Identity attempts to retrieve the node with the given identifier at the
	// selected point of the protocol state history. It will error if it doesn't exist.
	Identity(nodeID flow.Identifier) (*flow.Identity, error)

	// Commit return the sealed execution state commitment at this block.
	Commit() (flow.StateCommitment, error)

	// Pending returns the IDs of all descendants of the Head block. The IDs
	// are ordered such that parents are included before their children. These
	// are NOT guaranteed to have been validated by HotStuff.
	Pending() ([]flow.Identifier, error)

	// Seed returns a deterministic seed for a pseudo random number generator.
	// The seed is derived from the source of randomness for the Head block.
	// In order to deterministically derive task specific seeds, indices must
	// be specified. Refer to module/indices/rand.go for different indices.
	// NOTE: not to be confused with the epoch source of randomness!
	// error returns:
	//  * NoValidChildBlockError indicates that no valid child block is known
	//    (which contains the block's source of randomness)
	//  * unexpected errors should be considered symptoms of internal bugs
	Seed(indices ...uint32) ([]byte, error)

	// Phase returns the epoch phase for the current epoch, as of the Head block.
	Phase() (flow.EpochPhase, error)

	// Epochs returns a query object enabling querying detailed information about
	// various epochs.
	//
	// For epochs that are in the future w.r.t. the Head block, some of Epoch's
	// methods may return errors, since the Epoch Preparation Protocol may be
	// in-progress and incomplete for the epoch.
	Epochs() EpochQuery
}
