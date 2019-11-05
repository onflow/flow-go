// +build relic

package crypto

// Implements Joint Feldman protocol using BLS set up on BLS381 curve.
// the protocol runs (n) parallel instances of Feldman vss with the complaints system
// Private keys are Zr elements while public keys are G2 elements

type JointFeldmanState struct {
	*dkgCommon
	// jointRunning is true if all parallel Feldman vss protocols are running
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
	return nil, nil, nil, nil
}

// ReceiveDKGMsg processes a new DKG message received by the current node
func (s *JointFeldmanState) ReceiveDKGMsg(int, DKGmsg) *DKGoutput {
	return &DKGoutput{valid, nil, nil}
}

// Running returns the running state of Joint Feldman protocol
// It is true if all parallel Feldman vss protocols are running
func (s *JointFeldmanState) Running() bool {
	return s.jointRunning
}
