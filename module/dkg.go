// +build relic

package module

import (
	"context"

	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"google.golang.org/grpc"
)

// DKGSDKClientWrapper is a temporary solution to mocking the `sdk.Client` interface from `flow-go-sdk`
type DKGSDKClientWrapper interface {
	GetAccount(context.Context, sdk.Address, ...grpc.CallOption) (*sdk.Account, error)
	GetAccountAtLatestBlock(context.Context, sdk.Address, ...grpc.CallOption) (*sdk.Account, error)
	SendTransaction(context.Context, sdk.Transaction, ...grpc.CallOption) error
	GetLatestBlock(context.Context, bool, ...grpc.CallOption) (*sdk.Block, error)
	GetTransactionResult(context.Context, sdk.Identifier, ...grpc.CallOption) (*sdk.TransactionResult, error)
	ExecuteScriptAtBlockID(context.Context, flow.Identifier, []byte, []cadence.Value, ...grpc.CallOption) (cadence.Value, error)
}

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

	// SubmitResult submits the final public result of the DKG protocol. This
	// represents the group public key and the node's local computation of the
	// public keys for each DKG participant.
	//
	// SubmitResult must be called strictly after the final phase has ended.
	SubmitResult(crypto.PublicKey, []crypto.PublicKey) error
}

// DKGController controls the execution of a Joint Feldman DKG instance.
type DKGController interface {

	// Run starts the DKG controller and starts phase 1. It is a blocking call
	// that blocks until the controller is shutdown or until an error is
	// encountered in one of the protocol phases.
	Run() error

	// EndPhase0 notifies the controller to end phase 1, and start phase 2.
	EndPhase1() error

	// EndPhase1 notifies the controller to end phase 2, and start phase 3.
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
	Create(dkgInstanceID string, participants flow.IdentityList, seed []byte) (DKGController, error)
}

// DKGBroker extends the crypto.DKGProcessor interface with methods that enable
// a controller to access the channel of incoming messages, and actively fetch
// new DKG broadcast messages.
type DKGBroker interface {
	crypto.DKGProcessor

	// GetIndex returns the index of this node in the DKG committee list.
	GetIndex() int

	// GetPrivateMsgCh returns the channel through which a user can receive
	// incoming private DKGMessages.
	GetPrivateMsgCh() <-chan messages.DKGMessage

	// GetBroadcastMsgCh returns the channel through which a user can receive
	// incoming broadcast DKGMessages.
	GetBroadcastMsgCh() <-chan messages.DKGMessage

	// Poll instructs the broker to actively fetch broadcast messages (ex. read
	// from DKG smart contract). The messages will be forwarded through the
	// broker's message channel (cf. GetMsgCh). The method does not return until
	// all received messages are processed by the consumer.
	Poll(referenceBlock flow.Identifier) error

	// SubmitResult instructs the broker to publish the results of the DKG run
	// (ex. publish to DKG smart contract).
	SubmitResult(crypto.PublicKey, []crypto.PublicKey) error

	// Shutdown causes the broker to stop listening and forwarding messages.
	Shutdown()
}
