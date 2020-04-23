// +build relic

package crypto

// newSigner chooses and initializes a signature scheme
func newSigner(algo SigningAlgorithm) (signer, error) {
	if algo == BLSBLS12381 {
		return newBLSBLS12381(), nil
	}
	return newNonRelicSigner(algo)
}
