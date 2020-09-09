package protocol

import "github.com/dapperlabs/flow-go/model/flow"

type EpochState int

const (
	// Integer values: in many of our docs, we referred to the Staking Auction Phase as
	// 'Phase 0'. During Phase 0, no information about the upcoming Epoch is available
	// on the protocol level. Hence, we omit Phase 0 here.

	Setup     = 1 + iota // Epoch Preparation has a finalized Epoch Setup Event
	Committed            // Epoch Preparation has a finalized Epoch Committed Event
)

func (s EpochState) String() string {
	names := [...]string{
		"Setup",
		"Committed",
	}
	return names[s-1]
}

// EpochSnapshot contains the information specific to a certain Epoch (defined
// by the epoch Counter). Note that the Epoch preparation can differ along
// different forks. Therefore, an EpochSnapshot is tied to the Head of one
// specific fork and only accounts for the information contained in blocks
// along this fork up to (including) Head.
// An EpochSnapshot instance is constant and reports the identical information
// even if progress is made later and more information becomes available in
// subsequent blocks.
//
// Methods error if epoch preparation has not progressed far enough for
// this information to be determined by a finalized block.
type EpochSnapshot interface {

	// Counter returns the Epoch's counter.
	Counter() (uint64, error)

	// Head returns a block. An EpochSnapshot is tied to the Head (block)
	// of one specific fork and only accounts for the information contained
	// in blocks along this fork up to (including) Head.
	Head() (*flow.Header, error)

	// FinalView returns the largest view number which still belongs to this Epoch.
	FinalView() (uint64, error)

	// InitialIdentities returns the identities for this epoch as they were
	// specified in the EpochSetup Event.
	InitialIdentities(selector flow.IdentityFilter) (flow.IdentityList, error)

	Clustering() (flow.ClusterList, error)

	ClusterInformation(index uint32) (ClusterInformation, error)

	DKG() (DKG, error)

	EpochSetupSeed(indices ...uint32) ([]byte, error)

	//
	Phase() (EpochState, error)
}
