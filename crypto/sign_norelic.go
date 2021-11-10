// +build !relic

package crypto

// newSigner chooses and initializes a signature scheme
func newSigner(algo SigningAlgorithm) (signer, error) {
	return newNonRelicSigner(algo)
}

func init() {
	initNonRelic()
}

// VerifyPOP verifies a proof of possession (PoP) for the receiver public key; currently only works for BLS
func VerifyPOP(pk PublicKey, s Signature) (bool, error) {
	panic("VerifyPOP not supported with non-relic build")
}

// AggregateSignatures aggregate multiple signatures into one; currently only works for BLS
func AggregateSignatures(sigs [][]byte) (Signature, error) {
	panic("AggregateSignatures not supported with non-relic build")
}

// AggregatePublicKeys aggregate multiple public keys into one; currently only works for BLS
func AggregatePublicKeys(keys []PublicKey) (PublicKey, error) {
	panic("AggregatePublicKeys not supported with non-relic build")
}
