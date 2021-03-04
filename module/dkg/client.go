package dkg

// Client is a client to the Flow DKG contract. Allows functionality to Broadcast,
// read a Broadcast and submit the final result of the DKG protocol
type Client struct {
}

// NewClient initializes a new client to the Flow DKG contract
func NewClient() *Client {
	return &Client{}
}

// Broadcast broadcasts a message to all other nodes participating in the
// DKG. The message is broadcast by submitting a transaction to the DKG
// smart contract. An error is returned if the transaction has failed has
// failed
func (c *Client) Broadcast(msg messages.DKGMessage) error {
}

// ReadBroadcast reads the broadcast messages from the smart contract.
// Messages are returned in the order in which they were broadcast (received
// and stored in the smart contract)
func (c *Client) ReadBroadcast(fromIndex uint, referenceBlock flow.Identifier) ([]messages.DKGMessage, error) {
}

// SubmitResult submits the final public result of the DKG protocol. This
// represents the group public key and the node's local computation of the
// public keys for each DKG participant
func (c *Client) SubmitResult(crypto.PublicKey, []crypto.PublicKey) error {
}
