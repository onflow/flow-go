// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Local encapsulates the stable local node information.
type Local interface {

	// NodeID returns the node ID of the local node.
	NodeID() flow.Identifier

	// Address returns the (listen) address of the local node.
	Address() string

	// Sign provides a signature oracle that given a message and hasher, it
	// generates and returns a signature over the message using the node's private key
	// as well as the input hasher
	Sign([]byte, hash.Hasher) (crypto.Signature, error)
}
