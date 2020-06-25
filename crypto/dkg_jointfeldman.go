// +build relic

package crypto

// #cgo coverage CFLAGS: --coverage -fprofile-arcs -ftest-coverage -fprofile-instr-generate -fcoverage-mapping
// #cgo coverage LDFLAGS: --coverage -fprofile-instr-generate
// #cgo CFLAGS: -g -Wall -std=c99 -I./ -I./relic/include -I./relic/include/low
// #cgo LDFLAGS: -Lrelic/build/lib -l relic_s
// #include "dkg_include.h"
import "C"

import (
	"errors"
	"fmt"
)

// Implements Joint Feldman (Pedersen) protocol using
// the BLS set up on the BLS12-381 curve.
// The protocol runs (n) parallel instances of Feldman vss with
// the complaints mechanism, each participant being a leader
// once.

// This is a fully distributed generation. The secret is a BLS
// private key generated jointly by all the participants.

// In each feldman VSS istance, the leader generates a chunk of the
// the private key of a BLS threshold signature scheme.
// Using the complaints mechanism, each leader is qualified or disqualified
// from the protocol, and the overall key is taking into account
// all chunks from qualified leaders.

// Private keys are scalar in Zr, where r is the group order of G1/G2
// Public keys are in G2.

// Joint Feldman protocol, with complaint mechanism, implements DKGState
type JointFeldmanState struct {
	*dkgCommon
	// jointRunning is true if and only if all parallel Feldman vss protocols are running
	jointRunning bool
	// feldmanVSSQualState parallel states
	fvss []feldmanVSSQualState
	// is the group public key
	jointPublicKey pointG2
	// Private share of the current node
	jointx scalar
	// Public keys of the group nodes, the vector size is (n)
	jointy []pointG2
}

func (s *JointFeldmanState) init() {
	s.fvss = make([]feldmanVSSQualState, s.size)
	for i := 0; i < s.size; i++ {
		fvss := &feldmanVSSstate{
			dkgCommon:   s.dkgCommon,
			leaderIndex: index(i),
		}
		s.fvss[i] = feldmanVSSQualState{
			feldmanVSSstate: fvss,
			disqualified:    false,
		}
		s.fvss[i].init()
	}
}

// Start starts running Joint Feldman protocol in the current node
// The seed is used to generate the FVSS secret polynomial
// (including the instance group private key) when the current
// node is the leader.
func (s *JointFeldmanState) Start(seed []byte) error {
	if s.jointRunning {
		return errors.New("dkg is already running")
	}

	for i := index(0); int(i) < s.size; i++ {
		if i != s.currentIndex {
			s.fvss[i].running = false
			err := s.fvss[i].Start(seed)
			if err != nil {
				return fmt.Errorf("error when starting dkg: %w", err)
			}
		}
	}
	s.fvss[s.currentIndex].running = false
	err := s.fvss[s.currentIndex].Start(seed)
	if err != nil {
		return fmt.Errorf("error when starting dkg: %w", err)
	}
	s.jointRunning = true
	return nil
}

// NextTimeout sets the next timeout of the protocol if any timeout applies
func (s *JointFeldmanState) NextTimeout() error {
	if !s.jointRunning {
		return errors.New("dkg protocol is not running")
	}

	for i := index(0); int(i) < s.size; i++ {
		err := s.fvss[i].NextTimeout()
		if err != nil {
			return fmt.Errorf("next timeout has failed: %w", err)
		}
	}
	return nil
}

// End ends the protocol in the current node
// It returns the finalized public data and node private key share.
// - the group public key corresponding to the group secret key
// - all the public key shares corresponding to the nodes private
// key shares.
// - the finalized private key which is the current node's own private key share
func (s *JointFeldmanState) End() (PrivateKey, PublicKey, []PublicKey, error) {
	if !s.jointRunning {
		return nil, nil, nil, errors.New("dkg protocol is not running")
	}

	disqualifiedTotal := 0
	for i := 0; i < s.size; i++ {
		// check previous timeouts were called
		if !s.fvss[i].sharesTimeout || !s.fvss[i].complaintsTimeout {
			return nil, nil, nil,
				errors.New("two timeouts should be set before ending dkg")
		}

		// check if a complaint has remained without an answer
		// a leader is disqualified if a complaint was never answered
		if !s.fvss[i].disqualified {
			for _, c := range s.fvss[i].complaints {
				if c.received && !c.answerReceived {
					s.fvss[i].disqualified = true
					s.processor.Blacklist(i)
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
			errors.New("DKG has failed because the diqualified nodes number is high")
	}

	// wrap up the keys from qualified leaders
	jointx, jointPublicKey, jointy := s.sumUpQualifiedKeys(s.size - disqualifiedTotal)

	// private key of the current node
	x := &PrKeyBLSBLS12381{
		scalar: *jointx, // the private share
	}
	// Group public key
	Y := &PubKeyBLSBLS12381{
		point: *jointPublicKey,
	}
	// The nodes public keys
	y := make([]PublicKey, s.size)
	for i, p := range jointy {
		y[i] = &PubKeyBLSBLS12381{
			point: p,
		}
	}
	return x, Y, y, nil
}

// HandleMsg processes a new message received by the current node
// orig is the message origin index
func (s *JointFeldmanState) HandleMsg(orig int, msg []byte) error {
	if !s.jointRunning {
		return errors.New("dkg protocol is not running")
	}
	for i := index(0); int(i) < s.size; i++ {
		err := s.fvss[i].HandleMsg(orig, msg)
		if err != nil {
			return fmt.Errorf("handle message has failed: %w", err)
		}
	}
	return nil
}

// Running returns the running state of Joint Feldman protocol
func (s *JointFeldmanState) Running() bool {
	return s.jointRunning
}

// Disqualify forces a node to get disqualified
// for a reason outside of the DKG protocol
// The caller should make sure all honest nodes call this function,
// otherwise, the protocol can be broken
func (s *JointFeldmanState) Disqualify(node int) error {
	if !s.jointRunning {
		return errors.New("dkg is not running")
	}
	for i := 0; i < s.size; i++ {
		err := s.fvss[i].Disqualify(node)
		if err != nil {
			return fmt.Errorf("disqualif has failed: %w", err)
		}
	}
	return nil
}

// sum up the 3 type of keys from all qualified leaders to end the protocol
func (s *JointFeldmanState) sumUpQualifiedKeys(qualified int) (*scalar, *pointG2, []pointG2) {
	qualifiedx, qualifiedPubKey, qualifiedy := s.getQualifiedKeys(qualified)

	// sum up x
	var jointx scalar
	C.bn_sum_vector((*C.bn_st)(&jointx), (*C.bn_st)(&qualifiedx[0]),
		(C.int)(qualified))
	// sum up Y
	var jointPublicKey pointG2
	C.ep2_sum_vector((*C.ep2_st)(&jointPublicKey),
		(*C.ep2_st)(&qualifiedPubKey[0]), (C.int)(qualified))
	// sum up []y
	jointy := make([]pointG2, s.size)
	for i := 0; i < s.size; i++ {
		C.ep2_sum_vector((*C.ep2_st)(&jointy[i]),
			(*C.ep2_st)(&qualifiedy[i][0]), (C.int)(qualified))
	}
	return &jointx, &jointPublicKey, jointy
}

// get the 3 type of keys from all qualified leaders
func (s *JointFeldmanState) getQualifiedKeys(qualified int) ([]scalar, []pointG2, [][]pointG2) {
	qualifiedx := make([]scalar, 0, qualified)
	qualifiedPubKey := make([]pointG2, 0, qualified)
	qualifiedy := make([][]pointG2, s.size)
	for i := 0; i < s.size; i++ {
		qualifiedy[i] = make([]pointG2, 0, qualified)
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
