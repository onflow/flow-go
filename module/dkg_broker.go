//go:build relic
// +build relic

package module

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

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
