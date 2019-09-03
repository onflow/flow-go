package crypto

type feldmanVSSstate struct {
	// common DKG state
	*DKGcommon
	// node leader index
	leaderIndex int
	// internal context of BLS on BLS12381A
	blsContext *BLS_BLS12381Algo
	// Private share of the current node
	x scalar
}

func (s *feldmanVSSstate) init() {
	// Initialize BLS context
	s.blsContext := &(BLS_BLS12381Algo{
		nil,
		&SignAlgo{
			BLS_BLS12381,
			prKeyLengthBLS_BLS12381,
			pubKeyLengthBLS_BLS12381,
			signatureLengthBLS_BLS12381}})
	s.blsContext.init()
}

func (s *feldmanVSSstate) StartDKG() (DKGoutput, Error) {
	// Generate shares if necessary
	if s.leaderIndex == s.currentIndex {
		return s.generateShares()
	}
	return nil, nil
}

func (s *feldmanVSSstate) PrivateKey() PrivateKey {
	sk := PrKeyBLS_BLS12381{
		s.blsContext,  // signer algo
		nil,   		   // public key is not needed here
		s.x,		   // the private share
	}
	return &sk
}

func (s *feldmanVSSstate) generateShares() (DKGoutput, Error) {
	return nil, nil
}

func (s *feldmanVSSstate) receiveShare(in DKGinput) Error {
	return nil
}