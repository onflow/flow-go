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

<<<<<<< HEAD
// / EpochLookup provides a method to find epochs by view.
=======
// EpochLookup enables looking up epochs by view.
>>>>>>> feature/active-pacemaker
type EpochLookup interface {

	// EpochForViewWithFallback returns the counter of the epoch that the input view belongs to.
	// If epoch fallback has been triggered, returns the last committed epoch counter
	// in perpetuity for any inputs beyond the last committed epoch view range.
	// For example, if we trigger epoch fallback during epoch 10, and reach the final
	// view of epoch 10 before epoch 11 has finished being setup, this function will
	// return 10 even for input views beyond the final view of epoch 10.
	//
	// Returns model.ErrViewForUnknownEpoch if the input does not fall within the range of a known epoch.
	EpochForViewWithFallback(view uint64) (epochCounter uint64, err error)
}
