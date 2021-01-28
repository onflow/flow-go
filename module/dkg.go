package module

import (
	"github.com/onflow/flow-go/model/messages"
)

// DKGContractClient enables interacting with the DKG smart contract. This
// contract is deployed to the service account as part of a collection of
// smart contracts that facilitate and manage epoch transitions.
//
// TODO type for DKGResult? - should be all participant public keys plus group public key
type DKGContractClient interface {

	// Broadcast broadcasts a message to all other nodes participating in the
	// DKG. The message is broadcast by submitting a transaction to the DKG
	// smart contract. An error is returned if the transaction has failed and
	// should be re-submitted.
	Broadcast(msg messages.DKGMessage) error

	// ReadBroadcast reads the broadcast messages from the smart contract for
	// a particular phase. All messages for the given phase, with index greater
	// or equal to the offset, will be returned.
	//
	// ATTENTION: it is assumed that message indexes are kept on a per-phase
	// basis, ie. the first transaction of each phase has index 0.
	//
	// Messages are returned in the order in which they were broadcast (received
	// and stored in the smart contract).
	//
	// DKG nodes should call ReadBroadcast one final time once they have
	// observed the phase deadline trigger to guarantee they receive all
	// messages for that phase.
	ReadBroadcast(epochCounter uint64, phase messages.DKGPhase, offset int) ([]messages.DKGMessage, error)

	// SubmitResult submits the final public result of the DKG protocol. This
	// represents the node's local computation of the public keys for each
	// DKG participant and the group public key.
	//
	// SubmitResult must be called strictly after the final phase has ended.
	//
	// TODO type of DKGResult?
	SubmitResult(epochCounter uint64, result []byte) error
}
