package module

import (
	"context"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/state/protocol"
)

// ClusterRootQCVoter is responsible for submitting a vote to the cluster QC
// contract to coordinate generation of a valid root quorum certificate for the
// next epoch.
type ClusterRootQCVoter interface {

	// Vote handles the full procedure of generating a vote, submitting it to the
	// epoch smart contract, and verifying submission. Returns an error only if
	// there is a critical error that would make it impossible for the vote to be
	// submitted. Otherwise, exits when the vote has been successfully submitted.
	//
	// It is safe to run Vote multiple times within a single setup phase.
	Vote(context.Context, protocol.Epoch) error
}

// QCContractClient enables interacting with the cluster QC aggregator smart
// contract. This contract is deployed to the service account as part of a
// collection of smart contracts that facilitate and manage epoch transitions.
type QCContractClient interface {

	// SubmitVote submits the given vote to the cluster QC aggregator smart
	// contract. This function returns only once the transaction has been
	// processed by the network. An error is returned if the transaction has
	// failed and should be re-submitted.
	SubmitVote(ctx context.Context, vote *model.Vote) error

	// Voted returns true if we have successfully submitted a vote to the
	// cluster QC aggregator smart contract for the current epoch.
	Voted(ctx context.Context) (bool, error)
}

// EpochLookup provides a method to find epochs by view.
type EpochLookup interface {

	// EpochForView returns the counter of the epoch that the view belongs to.
	EpochForView(view uint64) (epochCounter uint64, err error)

	// EpochForViewWithFallback returns the counter of the epoch that the view belongs to.
	EpochForViewWithFallback(view uint64) (epochCounter uint64, err error)
}
