package crypto

// Implemets Feldman Verifiable Secret Sharing using BLS G2 group.

type feldmanVSSstate struct {
	// common DKG state
	*DKGcommon
	// node leader index
	leaderIndex int
	// internal context of BLS on BLS12381A
	blsContext *BLS_BLS12381Algo
	// Private share of the current node
	x scalar
	// Polynomial P = a_0 + a_1*x + .. + a_t*x^t  in Zr[X]
	// a_0 is the group private key
	a []scalar
	// Public keys of all the DKG nodes
	// A_0 is the group public key
	A []pointG2
}

func (s *feldmanVSSstate) init() {
	s.running = false

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

func (s *feldmanVSSstate) StartDKG() (*DKGoutput, error) {
	s.running = true
	// Generate shares if necessary
	if s.leaderIndex == s.currentIndex {
		return s.generateShares()
	}
	return new(DKGoutput), nil
}

func (s *feldmanVSSstate) EndDKG() error {
	s.running = false
	return nil
}

func (s *feldmanVSSstate) PrivateKey() PrKey {
	sk := PrKeyBLS_BLS12381{
		alg:    s.blsContext, // signer algo
		scalar: s.x,          // the private share
	}
	return &sk
}

func (s *feldmanVSSstate) generateShares() (*DKGoutput, error) {
	// Generate a polyomial P in Zr[X]
	s.a = make([]scalar, s.threshold+1)
	return new(DKGoutput), nil
}

func (s *feldmanVSSstate) receiveShare(in DKGinput) error {
	return nil
}
