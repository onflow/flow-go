package crypto

// Implemets Feldman Verifiable Secret Sharing using BLS G2 group.

type feldmanVSSstate struct {
	// common DKG state
	*DKGcommon
	// node leader index
	leaderIndex int
	// internal context of BLS on BLS12381A
	blsContext *BLS_BLS12381Algo
	// Polynomial P = a_0 + a_1*x + .. + a_t*x^t  in Zr[X], the vector size is (t+1)
	// a_0 is the group private key
	a []scalar
	// Public data of the group, the vector size is (t+1)
	// A_0 is the group public key
	A []pointG2
	// Private share of the current node
	x scalar
	// Public keys of the group nodes, the vector size is (n)
	y []pointG2
}

func (s *feldmanVSSstate) init() {
	s.isrunning = false

	// Initialize BLS context
	s.blsContext = &(BLS_BLS12381Algo{
		SignAlgo: &SignAlgo{
			name:            BLS_BLS12381,
			PrKeyLength:     prKeyLengthBLS_BLS12381,
			PubKeyLength:    pubKeyLengthBLS_BLS12381,
			SignatureLength: signatureLengthBLS_BLS12381,
		},
	})
	s.blsContext.init()

	s.A = make([]pointG2, s.threshold+1)
}

func (s *feldmanVSSstate) StartDKG() *DKGoutput {
	s.isrunning = true
	// Generate shares if necessary
	if s.leaderIndex == s.currentIndex {
		seed := []byte{1}
		return s.generateShares(seed)
	}
	return new(DKGoutput)
}

func (s *feldmanVSSstate) EndDKG() (PrKey, PubKey, []PubKey, error) {
	s.isrunning = false
	// private key of the current node
	x := PrKeyBLS_BLS12381{
		alg:    s.blsContext, // signer algo
		scalar: s.x,          // the private share
	}
	// Group public key
	Y := PubKeyBLS_BLS12381{
		point: s.A[0],
	}
	// The nodes public keys
	y := make([]PubKey, s.size)
	for i, p := range s.y {
		y[i] = &PubKeyBLS_BLS12381{
			point: p,
		}
	}
	return &x, &Y, y, nil
}

func (s *feldmanVSSstate) ProcessDKGmsg(orig int, msg DKGmsg) *DKGoutput {
	out := &(DKGoutput{
		action: []DKGToSend{},
		err:    nil,
	})

	msgBytes := msg[:]
	if len(msgBytes) == 0 {
		out.result = invalid
		return out
	}

	switch dkgMsgTag(msgBytes[0]) {
	case FeldmanVSSshare:
		out.result, out.err = s.receiveShare(orig, msgBytes[1:])
	default:
		out.result = invalid
	}
	return out
}
