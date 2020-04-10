// +build relic

package crypto

// newSigner chooses and initializes a signature scheme
func newSigner(algo SigningAlgorithm) (signer, error) {
	if algo == BLS_BLS12381 {
		return newBlsBLS12381(), nil
	}
	return newNonRelicSigner(algo)
}
