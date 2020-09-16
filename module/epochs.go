package module

import (
	"context"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
)

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
