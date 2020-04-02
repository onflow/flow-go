// +build relic

package crypto

import (
	"errors"
)

// Implements Joint Feldman protocol using BLS set up on BLS381 curve.
// the protocol runs (n) parallel instances of Feldman vss with the complaints system
// Private keys are Zr elements while public keys are G2 elements

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

// StartDKG starts running a DKG
func (s *JointFeldmanState) StartDKG(seed []byte) error {
	if s.jointRunning {
		return errors.New("dkg is already running")
	}

	for i := index(0); int(i) < s.size; i++ {
		if i != s.currentIndex {
			s.fvss[i].running = false
			err := s.fvss[i].StartDKG(seed)
			if err != nil {
				return errors.New("error when starting dkg")
			}
		}
	}
	s.fvss[s.currentIndex].running = false
	err := s.fvss[s.currentIndex].StartDKG(seed)
	if err != nil {
		return err
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
			return errors.New("next timeout has failed")
		}
	}
	return nil
}

// EndDKG ends a DKG protocol, the public data and node private key are finalized
func (s *JointFeldmanState) EndDKG() (PrivateKey, PublicKey, []PublicKey, error) {
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
	x := &PrKeyBLS_BLS12381{
		alg:    s.fvss[0].blsContext, // signer algo
		scalar: *jointx,              // the private share
	}
	// Group public key
	Y := &PubKeyBLS_BLS12381{
		alg:   s.fvss[0].blsContext,
		point: *jointPublicKey,
	}
	// The nodes public keys
	y := make([]PublicKey, s.size)
	for i, p := range jointy {
		y[i] = &PubKeyBLS_BLS12381{
			alg:   s.fvss[0].blsContext,
			point: p,
		}
	}
	return x, Y, y, nil
}

// ReceiveDKGMsg processes a new DKG message received by the current node
func (s *JointFeldmanState) ReceiveDKGMsg(orig int, msg []byte) error {
	if !s.jointRunning {
		return errors.New("dkg protocol is not running")
	}
	for i := index(0); int(i) < s.size; i++ {
		err := s.fvss[i].ReceiveDKGMsg(orig, msg)
		if err != nil {
			return errors.New("receive dkg message has failed")
		}
	}
	return nil
}

// Running returns the running state of Joint Feldman protocol
// It is true if all parallel Feldman vss protocols are running
func (s *JointFeldmanState) Running() bool {
	return s.jointRunning
}

func (s *JointFeldmanState) Disqualify(node int) error {
	if !s.jointRunning {
		return errors.New("dkg is not running")
	}
	for i := 0; i < s.size; i++ {
		err := s.fvss[i].Disqualify(node)
		if err != nil {
			return errors.New("disqualif has failed")
		}
	}
	return nil
}
