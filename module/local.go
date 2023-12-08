// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
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

	// NotMeFilter returns handy not-me filter for searching identity
	NotMeFilter() flow.IdentityFilter

	// SignFunc provides a signature oracle that given a message, a hasher, and a signing function, it
	// generates and returns a signature over the message using the node's private key
	// as well as the input hasher by invoking the given signing function. The overall idea of this function
	// is to not expose the private key to the caller.
	SignFunc([]byte, hash.Hasher, func(crypto.PrivateKey, []byte, hash.Hasher) (crypto.Signature,
		error)) (crypto.Signature, error)
}
