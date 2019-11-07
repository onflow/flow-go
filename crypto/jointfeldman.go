// +build relic

package crypto

import log "github.com/sirupsen/logrus"

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
func (s *JointFeldmanState) StartDKG(seed []byte) *DKGoutput {
	if !s.jointRunning {
		for i := index(0); int(i) < s.size; i++ {
			if i != s.currentIndex {
				s.fvss[i].running = false
				s.fvss[i].StartDKG(seed)
			}
		}
		//fmt.Println(s.currentIndex, s.fvss[s.currentIndex].leaderIndex, s.fvss[s.currentIndex].currentIndex)
		s.fvss[s.currentIndex].running = false
		out := s.fvss[s.currentIndex].StartDKG(seed)
		s.jointRunning = true
		return out
	}
	return &DKGoutput{valid, nil, nil}
}

// NextTimeout set the next timeout of the protocol if any timeout applies
func (s *JointFeldmanState) NextTimeout() *DKGoutput {
	out := &DKGoutput{
		result: valid,
		action: []DKGToSend{},
		err:    nil,
	}

	if !s.jointRunning {
		out.err = cryptoError{"dkg protocol is not running"}
		return out
	}

	for i := index(0); int(i) < s.size; i++ {
		loopOut := s.fvss[i].NextTimeout()
		if loopOut.err != nil {
			out.err = cryptoError{"next timeout has failed"}
		}
		if loopOut.action != nil {
			out.action = append(out.action, loopOut.action...)
		}
	}
	return out
}

// EndDKG ends a DKG protocol, the public data and node private key are finalized
func (s *JointFeldmanState) EndDKG() (PrivateKey, PublicKey, []PublicKey, error) {
	if !s.jointRunning {
		return nil, nil, nil, cryptoError{"dkg protocol is not running"}
	}

	disqualifiedTotal := 0
	for i := 0; i < s.size; i++ {
		// check previous timeouts were called
		if !s.fvss[i].sharesTimeout || !s.fvss[i].complaintsTimeout {
			return nil, nil, nil,
				cryptoError{"two timeouts should be set before ending dkg"}
		}

		// check if a complaint has remained without an answer
		// a leader is disqualified if a complaint was never answered
		if !s.fvss[i].disqualified {
			for _, c := range s.fvss[i].complaints {
				if c.received && !c.answerReceived {
					s.fvss[i].disqualified = true
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
	log.Infof("node %d has disqualified %d other nodes", s.currentIndex, disqualifiedTotal)
	if disqualifiedTotal > s.threshold || s.size-disqualifiedTotal <= s.threshold {
		return nil, nil, nil,
			cryptoError{"DKG has failed because the diqualified nodes number is high"}
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
		point: *jointPublicKey,
	}
	// The nodes public keys
	y := make([]PublicKey, s.size)
	for i, p := range jointy {
		y[i] = &PubKeyBLS_BLS12381{
			point: p,
		}
	}
	return x, Y, y, nil
}

// ReceiveDKGMsg processes a new DKG message received by the current node
func (s *JointFeldmanState) ReceiveDKGMsg(orig int, msg DKGmsg) *DKGoutput {
	out := &DKGoutput{
		result: valid,
		action: []DKGToSend{},
		err:    nil,
	}

	if !s.jointRunning {
		out.result = invalid
		return out
	}
	for i := index(0); int(i) < s.size; i++ {
		loopOut := s.fvss[i].ReceiveDKGMsg(orig, msg)
		if loopOut.err != nil {
			out.err = cryptoError{"receive dkg message has failed"}
		}
		if loopOut.action != nil {
			out.action = append(out.action, loopOut.action...)
		}
	}
	return out
}

// Running returns the running state of Joint Feldman protocol
// It is true if all parallel Feldman vss protocols are running
func (s *JointFeldmanState) Running() bool {
	return s.jointRunning
}
