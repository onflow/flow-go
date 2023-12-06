package crypto

// #include "dkg_include.h"
import "C"

import (
	"fmt"
)

// Implements Joint Feldman (Pedersen) protocol using
// the BLS set up on the BLS12-381 curve.
// The protocol runs (n) parallel instances of Feldman vss with
// the complaints mechanism, each participant being a dealer
// once.

// This is a fully distributed generation. The secret is a BLS
// private key generated jointly by all the participants.

// (t) is the threshold parameter. Although the API allows using arbitrary values of (t),
// the DKG protocol is secure in the presence of up to (t) malicious participants
// when (t < n/2).
// Joint-Feldman is the protocol implemented in Flow, (t) being set to the maximum value
// t = floor((n-1)/2) to optimize for unforgeability and robustness of the threshold
// signature scheme using the output keys.

// In each feldman VSS instance, the dealer generates a chunk of the
// the private key of a BLS threshold signature scheme.
// Using the complaints mechanism, each dealer is qualified or disqualified
// from the protocol, and the overall key is taking into account
// all chunks from qualified dealers.

// Private keys are scalar in Fr, where r is the group order of G1/G2
// Public keys are in G2.

// Joint Feldman protocol, with complaint mechanism, implements DKGState
type JointFeldmanState struct {
	*dkgCommon
	// jointRunning is true if and only if all parallel Feldman vss protocols are running
	jointRunning bool
	// feldmanVSSQualState parallel states
	fvss []feldmanVSSQualState
	// is the group public key
	jointPublicKey pointE2
	// Private share of the current participant
	jointx scalar
	// Public keys of the group participants, the vector size is (n)
	jointy []pointE2
}

// NewJointFeldman creates a new instance of a Joint Feldman protocol.
//
// size if the total number of participants (n).
// threshold is the threshold parameter (t). the DKG protocol is secure in the
// presence of up to (t) malicious participants when (t < n/2).
// myIndex is the index of the participant creating the new DKG instance.
// processor is the DKGProcessor instance required to connect the participant to the
// communication channels.
//
// An instance is run by a single participant and is usable for only one protocol.
// In order to run the protocol again, a new instance needs to be created.
//
// The function returns:
// - (nil, InvalidInputsError) if:
//   - size if not in [DKGMinSize, DKGMaxSize]
//   - threshold is not in [MinimumThreshold, size-1]
//   - myIndex is not in [0, size-1]
//   - dealerIndex is not in [0, size-1]
//
// - (dkgInstance, nil) otherwise
func NewJointFeldman(size int, threshold int, myIndex int,
	processor DKGProcessor) (DKGState, error) {

	common, err := newDKGCommon(size, threshold, myIndex, processor, 0)
	if err != nil {
		return nil, err
	}

	jf := &JointFeldmanState{
		dkgCommon: common,
	}
	jf.init()
	return jf, nil
}

func (s *JointFeldmanState) init() {
	s.fvss = make([]feldmanVSSQualState, s.size)
	for i := 0; i < s.size; i++ {
		fvss := &feldmanVSSstate{
			dkgCommon:   s.dkgCommon,
			dealerIndex: index(i),
		}
		s.fvss[i] = feldmanVSSQualState{
			feldmanVSSstate: fvss,
			disqualified:    false,
		}
		s.fvss[i].init()
	}
}

// Start triggers Joint Feldman protocol start for the current participant.
// The seed is used to generate the FVSS secret polynomial
// (including the instance group private key) when the current
// participant is the dealer.
//
// The returned error is :
//   - dkgInvalidStateTransitionError if the DKG instance is already running.
//   - error if an unexpected exception occurs
//   - nil otherwise.
func (s *JointFeldmanState) Start(seed []byte) error {
	if s.jointRunning {
		return dkgInvalidStateTransitionErrorf("dkg is already running")
	}

	for i := index(0); int(i) < s.size; i++ {
		s.fvss[i].running = false
		err := s.fvss[i].Start(seed)
		if err != nil {
			return fmt.Errorf("error when starting dkg: %w", err)
		}
	}
	s.jointRunning = true
	return nil
}

// NextTimeout sets the next timeout of the protocol if any timeout applies.
//
// The returned error is :
//   - dkgInvalidStateTransitionError if the DKG instance was not running.
//   - dkgInvalidStateTransitionError if the DKG instance already called the 2 required timeouts.
//   - nil otherwise.
func (s *JointFeldmanState) NextTimeout() error {
	if !s.jointRunning {
		return dkgInvalidStateTransitionErrorf("dkg protocol %d is not running", s.myIndex)
	}

	for i := index(0); int(i) < s.size; i++ {
		err := s.fvss[i].NextTimeout()
		if err != nil {
			return fmt.Errorf("next timeout failed: %w", err)
		}
	}
	return nil
}

// End ends the protocol in the current participant
// It returns the finalized public data and participant private key share.
// - the group public key corresponding to the group secret key
// - all the public key shares corresponding to the participants private
// key shares.
// - the finalized private key which is the current participant's own private key share
//
// The returned error is:
//   - dkgFailureError if the disqualified dealers exceeded the threshold
//   - dkgFailureError if the public key share or group public key is identity.
//   - dkgInvalidStateTransitionError Start() was not called, or NextTimeout() was not called twice
//   - nil otherwise.
func (s *JointFeldmanState) End() (PrivateKey, PublicKey, []PublicKey, error) {
	if !s.jointRunning {
		return nil, nil, nil, dkgInvalidStateTransitionErrorf("dkg protocol %d is not running", s.myIndex)
	}

	disqualifiedTotal := 0
	for i := 0; i < s.size; i++ {
		// check previous timeouts were called
		if !s.fvss[i].sharesTimeout || !s.fvss[i].complaintsTimeout {
			return nil, nil, nil,
				dkgInvalidStateTransitionErrorf("%d: two timeouts should be set before ending dkg", s.myIndex)
		}

		// check if a complaint has remained without an answer
		// a dealer is disqualified if a complaint was never answered
		if !s.fvss[i].disqualified {
			for complainer, c := range s.fvss[i].complaints {
				if c.received && !c.answerReceived {
					s.fvss[i].disqualified = true
					s.processor.Disqualify(i,
						fmt.Sprintf("complaint from %d was not answered", complainer))
					disqualifiedTotal++
					break
				}
			}
		} else {
			disqualifiedTotal++
		}
	}
	s.jointRunning = false

	// check failing dkg
	if disqualifiedTotal > s.threshold || s.size-disqualifiedTotal <= s.threshold {
		return nil, nil, nil,
			dkgFailureErrorf(
				"Joint-Feldman failed because the disqualified participants number is high: %d disqualified, threshold is %d, size is %d",
				disqualifiedTotal, s.threshold, s.size)
	}

	// wrap up the keys from qualified dealers
	jointx, jointPublicKey, jointy := s.sumUpQualifiedKeys(s.size - disqualifiedTotal)

	// private key of the current participant
	x := newPrKeyBLSBLS12381(jointx)

	// Group public key
	Y := newPubKeyBLSBLS12381(jointPublicKey)

	// The participants public keys
	y := make([]PublicKey, s.size)
	for i, p := range jointy {
		y[i] = newPubKeyBLSBLS12381(&p)
	}

	// check if current public key share or group public key is identity.
	// In that case all signatures generated by the current private key share or
	// the group private key are invalid (as stated by the BLS IETF draft)
	// to avoid equivocation issues.
	//
	// Assuming both private keys have entropy from at least one honest dealer, each private
	// key is initially uniformly distributed over the 2^255 possible values. We can argue that
	// the known uniformity-bias caused by malicious dealers in Joint-Feldman does not weaken
	// the likelihood of generating an identity key to practical probabilities.
	if (jointx).isZero() {
		return nil, nil, nil, dkgFailureErrorf("private key share is identity and is therefore invalid")
	}
	if Y.isIdentity {
		return nil, nil, nil, dkgFailureErrorf("group private key is identity and is therefore invalid")
	}
	return x, Y, y, nil
}

// HandleBroadcastMsg processes a new broadcasted message received by the current participant
// orig is the message origin index
//
// The function returns:
//   - dkgInvalidStateTransitionError if the instance is not running
//   - invalidInputsError if `orig` is not valid (in [0, size-1])
//   - nil otherwise
func (s *JointFeldmanState) HandleBroadcastMsg(orig int, msg []byte) error {
	if !s.jointRunning {
		return dkgInvalidStateTransitionErrorf("dkg protocol %d is not running", s.myIndex)
	}
	for i := index(0); int(i) < s.size; i++ {
		err := s.fvss[i].HandleBroadcastMsg(orig, msg)
		if err != nil {
			return fmt.Errorf("handle broadcast message failed: %w", err)
		}
	}
	return nil
}

// HandlePrivateMsg processes a new private message received by the current participant
// orig is the message origin index
//
// The function returns:
//   - dkgInvalidStateTransitionError if the instance is not running
//   - invalidInputsError if `orig` is not valid (in [0, size-1])
//   - nil otherwise
func (s *JointFeldmanState) HandlePrivateMsg(orig int, msg []byte) error {
	if !s.jointRunning {
		return dkgInvalidStateTransitionErrorf("dkg protocol %d is not running", s.myIndex)
	}
	for i := index(0); int(i) < s.size; i++ {
		err := s.fvss[i].HandlePrivateMsg(orig, msg)
		if err != nil {
			return fmt.Errorf("handle private message failed: %w", err)
		}
	}
	return nil
}

// Running returns the running state of Joint Feldman protocol
func (s *JointFeldmanState) Running() bool {
	return s.jointRunning
}

// ForceDisqualify forces a participant to get disqualified
// for a reason outside of the DKG protocol
// The caller should make sure all honest participants call this function,
// otherwise, the protocol can be broken
//
// The function returns:
//   - dkgInvalidStateTransitionError if the instance is not running
//   - invalidInputsError if `orig` is not valid (in [0, size-1])
//   - nil otherwise
func (s *JointFeldmanState) ForceDisqualify(participant int) error {
	if !s.jointRunning {
		return dkgInvalidStateTransitionErrorf("dkg is not running")
	}
	// disqualify the participant in the fvss instance where they are a dealer
	err := s.fvss[participant].ForceDisqualify(participant)
	if err != nil {
		return fmt.Errorf("force disqualify failed: %w", err)
	}
	return nil
}

// sum up the 3 type of keys from all qualified dealers to end the protocol
func (s *JointFeldmanState) sumUpQualifiedKeys(qualified int) (*scalar, *pointE2, []pointE2) {
	qualifiedx, qualifiedPubKey, qualifiedy := s.getQualifiedKeys(qualified)

	// sum up x
	var jointx scalar
	C.Fr_sum_vector((*C.Fr)(&jointx), (*C.Fr)(&qualifiedx[0]),
		(C.int)(qualified))
	// sum up Y
	var jointPublicKey pointE2
	C.E2_sum_vector_to_affine((*C.E2)(&jointPublicKey),
		(*C.E2)(&qualifiedPubKey[0]), (C.int)(qualified))
	// sum up []y
	jointy := make([]pointE2, s.size)
	for i := 0; i < s.size; i++ {
		C.E2_sum_vector_to_affine((*C.E2)(&jointy[i]),
			(*C.E2)(&qualifiedy[i][0]), (C.int)(qualified))
	}
	return &jointx, &jointPublicKey, jointy
}

// get the 3 type of keys from all qualified dealers
func (s *JointFeldmanState) getQualifiedKeys(qualified int) ([]scalar, []pointE2, [][]pointE2) {
	qualifiedx := make([]scalar, 0, qualified)
	qualifiedPubKey := make([]pointE2, 0, qualified)
	qualifiedy := make([][]pointE2, s.size)
	for i := 0; i < s.size; i++ {
		qualifiedy[i] = make([]pointE2, 0, qualified)
	}

	for i := 0; i < s.size; i++ {
		if !s.fvss[i].disqualified {
			qualifiedx = append(qualifiedx, s.fvss[i].x)
			qualifiedPubKey = append(qualifiedPubKey, s.fvss[i].vA[0])
			for j := 0; j < s.size; j++ {
				qualifiedy[j] = append(qualifiedy[j], s.fvss[i].y[j])
			}
		}
	}
	return qualifiedx, qualifiedPubKey, qualifiedy
}
