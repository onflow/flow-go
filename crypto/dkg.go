// +build relic

package crypto

// DKG stands for distributed key generation. In this library, DKG
// refers discrete-log based protocols that generate keys for a BLS-based
// threshold signature scheme.
// BLS is used with the BLS12381 curve.

// These protocols mainly generate a BLS key pair and share the secret key
// among (n) participants in a way that any (t+1) key shares allow reconstructing
// the initial key (and also reconstructing a BLS signature of the initial key).
// We refer to the initial key pair by group private and group public key.
// (t) is the threshold parameter and is fixed for all protocols to maximize
// the unforgeability and robustness, with a value of t = floor((n-1)/2).

// Private keys are scalar in Zr, where r is the group order of G1/G2.
// Public keys are in G2.

import (
	"fmt"
)

// DKGProtocol is the type for supported protocols
type DKGProtocol int

// Supported Key Generation protocols
const (
	// FeldmanVSS is Feldman Verifiable Secret Sharing
	// (non distributed generation)
	FeldmanVSS DKGProtocol = iota
	// FeldmanVSSQual is Feldman Verifiable Secret Sharing using a complaint
	// mechanism to qualify/disqualify the leader
	// (non distributed generation)
	FeldmanVSSQual
	// Joint Feldman (Pedersen) is a protocol made of (n) parallel Feldman VSS
	// with a complaint mechanism
	// (distributed generation)
	JointFeldman
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

// optimal threshold (t) to allow the largest number of malicious nodes (m)
// assuming the protocol requires:
//   m<=t for unforgeability
//   n-m<=t+1 for robustness
func optimalThreshold(size int) int {
	return (size - 1) / 2
}

// NewDKG creates a new instance of a DKG protocol.
//
// An instance is run by a single node and is usable for only one protocol.
// In order to run the protocol again, a new instance needs to be created
// leaderIndex value is ignored if the protocol does not require a leader (JointFeldman for instance)
func NewDKG(dkg DKGProtocol, size int, currentIndex int,
	processor DKGProcessor, leaderIndex int) (DKGState, error) {
	if size < DKGMinSize || size > DKGMaxSize {
		return nil, fmt.Errorf("size should be between %d and %d", DKGMinSize, DKGMaxSize)
	}

	if currentIndex >= size || leaderIndex >= size || currentIndex < 0 || leaderIndex < 0 {
		return nil, fmt.Errorf("indices of current and leader nodes must be between 0 and %d", size-1)
	}

	// optimal threshold (t) to allow the largest number of malicious nodes (m)
	threshold := optimalThreshold(size)
	common := &dkgCommon{
		size:         size,
		threshold:    threshold,
		currentIndex: index(currentIndex),
		processor:    processor,
	}
	switch dkg {
	case FeldmanVSS:
		fvss := &feldmanVSSstate{
			dkgCommon:   common,
			leaderIndex: index(leaderIndex),
		}
		fvss.init()
		return fvss, nil
	case FeldmanVSSQual:
		fvss := &feldmanVSSstate{
			dkgCommon:   common,
			leaderIndex: index(leaderIndex),
		}
		fvssq := &feldmanVSSQualState{
			feldmanVSSstate: fvss,
			disqualified:    false,
		}
		fvssq.init()
		return fvssq, nil
	case JointFeldman:
		jf := &JointFeldmanState{
			dkgCommon: common,
		}
		jf.init()
		return jf, nil
	default:
		return nil, fmt.Errorf("the Distributed Key Generation %d is not supported", dkg)
	}
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
	// PrivateSend sends a private message to a destination.
	// The data to be sent is confidential and the function
	// must make sure data is encrypted before being shared
	// on a public channel, in a way that it is only received
	// by the destination participant.
	PrivateSend(dest int, data []byte)
	// Broadcast broadcasts a message to all participants.
	// This function assumes all nodes have received the same message,
	// failing to do so, the protocol can be broken.
	// The broadcasted message is public and not confidential.
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
