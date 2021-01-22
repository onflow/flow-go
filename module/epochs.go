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

// DKGContractClient enables interacting with the DKG smart contract. This
// contract is deployed to the service account as part of a collection of
// smart contracts that facilitate and manage epoch transitions.
//
// Phase parameters may be one of [1,2,3].
//
// TODO type for DKGPhase?
// TODO type for DKGMessage?
// TODO type for DKGResult? - should be all participant public keys plus group public key
type DKGContractClient interface {

	// Broadcast broadcasts a message to all other nodes participating in the
	// DKG for a particular phase. The message is broadcast by submitting a
	// transaction to the DKG smart contract. An error is returned if the
	// transaction has failed and should be re-submitted.
	//
	// Messages submitted for a phase which has ended will not be accepted and
	// will result in an error.
	Broadcast(phase int, msg []byte) error

	// ReadBroadcast reads the broadcast messages from the smart contract for
	// a particular phase. All messages for the given phase that have been
	// successfully broadcast will be returned, regardless of whether those
	// messages have already been read by a previous call to ReadBroadcast.
	//
	// Messages are returned in the order in which they were broadcast (received
	// and stored in the smart contract).
	//
	// DKG nodes should call ReadBroadcast one final time once they have
	// observed the phase deadline trigger to guarantee they receive all
	// messages for that phase.
	//
	// OPTIONAL: add a messageIndex parameter to only receive messages we have
	// not already received in previous calls
	ReadBroadcast(phase int) ([][]byte, error)

	// SubmitResult submits the final public result of the DKG protocol. This
	// represents the node's local computation of the public keys for each
	// DKG participant and the group public key.
	// TODO type of DKGResult?
	SubmitResult(result []byte) error
}
