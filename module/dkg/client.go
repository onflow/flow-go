package dkg

import (
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

// Client is a client to the Flow DKG contract. Allows functionality to Broadcast,
// read a Broadcast and submit the final result of the DKG protocol
type Client struct {
	accessAddress      string
	dkgContractAddress string
	accountAddress     string
	accountKey         uint
	flowClient         *client.Client
	signer             sdkcrypto.Signer
}

// NewClient initializes a new client to the Flow DKG contract
func NewClient(accessAddress, dkgContractAddress, accountAddress string, accountKeyIndex uint, flowClient *client.Client, signer sdkcrypto.Signer) *Client {
	return &Client{
		accessAddress:      accessAddress,
		flowClient:         flowClient,
		dkgContractAddress: dkgContractAddress,
		signer:             signer,
		accountKey:         accountKeyIndex,
		accountAddress:     accountAddress,
	}
}

// Broadcast broadcasts a message to all other nodes participating in the
// DKG. The message is broadcast by submitting a transaction to the DKG
// smart contract. An error is returned if the transaction has failed has
// failed
func (c *Client) Broadcast(msg messages.DKGMessage) error {
	tx := sdk.NewTransaction().
		SetScript(templates.GenerateSendDKGMessage()).
		SetGasLimit(9999).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(authorizerAddress)

	return nil
}

// ReadBroadcast reads the broadcast messages from the smart contract.
// Messages are returned in the order in which they were broadcast (received
// and stored in the smart contract)
func (c *Client) ReadBroadcast(fromIndex uint, referenceBlock flow.Identifier) ([]messages.DKGMessage, error) {
	return []messages.DKGMessage{}, nil
}

// SubmitResult submits the final public result of the DKG protocol. This
// represents the group public key and the node's local computation of the
// public keys for each DKG participant
func (c *Client) SubmitResult(crypto.PublicKey, []crypto.PublicKey) error {
	return nil
}
