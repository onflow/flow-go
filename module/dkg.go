package module

import (
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

// DKGContractClient enables interacting with the DKG smart contract. This
// contract is deployed to the service account as part of a collection of
// smart contracts that facilitate and manage epoch transitions.
type DKGContractClient interface {

	// Broadcast broadcasts a message to all other nodes participating in the
	// DKG. The message is broadcast by submitting a transaction to the DKG
	// smart contract. An error is returned if the transaction has failed has
	// failed.
	// TBD: retry logic
	Broadcast(msg messages.BroadcastDKGMessage) error

	// ReadBroadcast reads the broadcast messages from the smart contract.
	// Messages are returned in the order in which they were broadcast (received
	// and stored in the smart contract). The parameters are:
	//
	// * fromIndex: return messages with index >= fromIndex
	// * referenceBlock: a marker for the state against which the query should
	//   be executed
	//
	// DKG nodes should call ReadBroadcast one final time once they have
	// observed the phase deadline trigger to guarantee they receive all
	// messages for that phase.
	ReadBroadcast(fromIndex uint, referenceBlock flow.Identifier) ([]messages.BroadcastDKGMessage, error)

	// SubmitParametersAndResult posts the DKG setup parameters (`flow.DKGIndexMap`) and the node's locally-computed DKG result to
	// the DKG white-board smart contract. The DKG results are the node's local computation of the group public key and the public
	// key shares. Serialized public keys are encoded as lower-case hex strings.
	// Conceptually the flow.DKGIndexMap is not an output of the DKG protocol. Rather, it is part of the configuration/initialization
	// information of the DKG. Before an epoch transition on the happy path (using the data in the EpochSetup event), each consensus
	// participant locally fixes the DKG committee 𝒟 including the respective nodes' order to be identical to the consensus
	// committee 𝒞. However, in case of a failed epoch transition, we desire the ability to manually provide the result of a successful
	// DKG for the immediately next epoch (so-called recovery epoch). The DKG committee 𝒟 must have a sufficiently large overlap with
	// the recovery epoch's consensus committee 𝒞 -- though for flexibility, we do *not* want to require that both committees are identical.
	// Therefore, we need to explicitly specify the DKG committee 𝒟 on the fallback path. For uniformity of implementation, we do the
	// same also on the happy path.
	SubmitParametersAndResult(indexMap flow.DKGIndexMap, groupPublicKey crypto.PublicKey, publicKeys []crypto.PublicKey) error

	// SubmitEmptyResult submits an empty result of the DKG protocol.
	// The empty result is obtained by a node when it realizes locally that its DKG participation
	// was unsuccessful (possible reasons include: node received too many byzantine inputs;
	// node has networking issues; locally computed key is invalid…). However, a node obtaining an
	// empty result can happen in both cases of the DKG succeeding or failing globally.
	// For further details, please see:
	// https://flowfoundation.notion.site/Random-Beacon-2d61f3b3ad6e40ee9f29a1a38a93c99c
	// Honest nodes would call `SubmitEmptyResult` strictly after the final phase has ended if DKG has ended.
	// However, `SubmitEmptyResult` also supports implementing byzantine participants for testing that submit an
	// empty result too early (intentional protocol violation), *before* the final DKG phase concluded.
	SubmitEmptyResult() error
}

// DKGController controls the execution of a Joint Feldman DKG instance.
type DKGController interface {

	// Run starts the DKG controller and starts phase 1. It is a blocking call
	// that blocks until the controller is shutdown or until an error is
	// encountered in one of the protocol phases.
	Run() error

	// EndPhase1 notifies the controller to end phase 1, and start phase 2.
	EndPhase1() error

	// EndPhase2 notifies the controller to end phase 2, and start phase 3.
	EndPhase2() error

	// End terminates the DKG state machine and records the artifacts.
	End() error

	// Shutdown stops the controller regardless of the current state.
	Shutdown()

	// Poll instructs the controller to actively fetch broadcast messages (ex.
	// read from DKG smart contract). The method does not return until all
	// received messages are processed.
	Poll(blockReference flow.Identifier) error

	// GetArtifacts returns our node's private key share, the group public key,
	// and the list of all nodes' public keys (including ours), as computed by
	// the DKG.
	GetArtifacts() (crypto.PrivateKey, crypto.PublicKey, []crypto.PublicKey)

	// GetIndex returns the index of this node in the DKG committee list.
	GetIndex() int

	// SubmitResult instructs the broker to publish the results of the DKG run
	// (ex. publish to DKG smart contract).
	SubmitResult() error
}

// DKGControllerFactory is a factory to create instances of DKGController.
type DKGControllerFactory interface {

	// Create instantiates a new DKGController.
	Create(dkgInstanceID string, participants flow.IdentitySkeletonList, seed []byte) (DKGController, error)
}
