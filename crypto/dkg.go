// +build relic

package crypto

// DKG stands for distributed key generation. In this library, DKG
// refers to discrete-log based protocols that generate keys for a BLS-based
// threshold signature scheme.
// BLS is used with the BLS12-381 curve.

// These protocols mainly generate a BLS key pair and share the secret key
// among (n) participants in a way that any (t+1) key shares allow reconstructing
// the initial key (and also reconstructing a BLS threshold signature under the initial key).
// We refer to the initial key pair by group private and group public key.
// (t) is the threshold parameter. Although the API allows using arbitrary values of (t),
// Flow uses DKG with the value t = floor((n-1)/2) to optimize for unforgeability and robustness
// of the threshold signature scheme using the output keys.

// Private keys are scalar in Zr, where r is the group order of G1/G2.
// Public keys are in G2.

import (
	"fmt"
)

type DKGState interface {
	// Size returns the size of the DKG group n
	Size() int
	// Threshold returns the threshold value t
	Threshold() int
	// Start starts running a DKG in the current node
	Start(seed []byte) error
	// HandleMsg processes a new message received by the current node.
	// orig is the message origin index
	HandleMsg(orig int, msg []byte) error
	// End ends a DKG protocol in the current node.
	// It returns the finalized public data and node private key share.
	// - the group public key corresponding to the group secret key
	// - all the public key shares corresponding to the nodes private
	// key shares
	// - the finalized private key which is the current node's own private key share
	End() (PrivateKey, PublicKey, []PublicKey, error)
	// NextTimeout set the next timeout of the protocol if any timeout applies.
	// Some protocols could require more than one timeout
	NextTimeout() error
	// Running returns the running state of the DKG protocol
	Running() bool
	// Disqualify forces a node to get disqualified
	// for a reason outside of the DKG protocol.
	// The caller should make sure all honest nodes call this function,
	// otherwise, the protocol can be broken.
	Disqualify(node int) error
}

// index is the node index type used as participants ID
type index byte

// newDKGCommon initializes the common structure of DKG protocols
func newDKGCommon(size int, threshold int, currentIndex int,
	processor DKGProcessor, leaderIndex int) (*dkgCommon, error) {
	if size < DKGMinSize || size > DKGMaxSize {
		return nil, fmt.Errorf("size should be between %d and %d", DKGMinSize, DKGMaxSize)
	}

	if currentIndex >= size || leaderIndex >= size || currentIndex < 0 || leaderIndex < 0 {
		return nil, fmt.Errorf("indices of current and leader nodes must be between 0 and %d", size-1)
	}

	if threshold >= size || threshold < 0 {
		return nil, fmt.Errorf("The threshold must be between 0 and %d", size-1)
	}

	return &dkgCommon{
		size:         size,
		threshold:    threshold,
		currentIndex: index(currentIndex),
		processor:    processor,
	}, nil
}

// dkgCommon holds the common data of all DKG protocols
type dkgCommon struct {
	size         int
	threshold    int
	currentIndex index
	// running is true when the DKG protocol is running, is false otherwise
	running bool
	// processes the action of the DKG interface outputs
	processor DKGProcessor
}

// Running returns the running state of the DKG protocol.
// The state is equal to true when the DKG protocol is running, and is equal to false otherwise.
func (s *dkgCommon) Running() bool {
	return s.running
}

// Size returns the size of the DKG group n
func (s *dkgCommon) Size() int {
	return s.size
}

// Threshold returns the threshold value t
func (s *dkgCommon) Threshold() int {
	return s.threshold
}

// NextTimeout sets the next protocol timeout if there is any.
// This function should be overwritten by any protocol that uses timeouts.
func (s *dkgCommon) NextTimeout() error {
	return nil
}

// dkgMsgTag is the type used to encode message tags
type dkgMsgTag byte

const (
	feldmanVSSShare dkgMsgTag = iota
	feldmanVSSVerifVec
	feldmanVSSComplaint
	feldmanVSSComplaintAnswer
)

// DKGProcessor is an interface that implements the DKG output actions.
//
// An instance of a DKGProcessor is needed for each node in order to
// particpate in a DKG protocol
type DKGProcessor interface {
	// PrivateSend sends a message to a destination over
	// a private channel. The channel must preserve the
	// confidentiality of the message and should authenticate
	// the sender.
	// It is recommended that the private channel is unique per
	// protocol instance. This can be achieved by prepending all
	// messages by a unique instance ID.
	PrivateSend(dest int, data []byte)
	// Broadcast broadcasts a message to all participants.
	// This function assumes all nodes have received the same message,
	// failing to do so, the protocol can be broken.
	// The broadcasted message is public and not confidential.
	// The broadcasting channel should authenticate the sender.
	// It is recommended that the broadcasting channel is unique per
	// protocol instance. This can be achieved by prepending all
	// messages by a unique instance ID.
	Broadcast(data []byte)
	// Blacklist flags that a node is misbehaving and that it got
	// disqualified from the protocol. Such behavior deserves
	// disqualifying as it is flagged to all honest nodes in
	// the protocol.
	Blacklist(node int)
	// FlagMisbehavior warns that a node is misbehaving.
	// Such behavior is not necessarily flagged to all nodes and therefore
	// the node is not disqualified from the protocol. Other mechanisms
	// outside DKG could be implemented to synchronize slashing the misbehaving
	// node by all participating nodes, using the api `Disqualify`. Failing to
	// do so, the protocol can be broken.
	FlagMisbehavior(node int, log string)
}

const (
	wrongFormat   = "wrong message format"
	duplicated    = "message type is duplicated"
	wrongProtocol = "message is not compliant with the protocol"
)
