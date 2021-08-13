// +build !relic

package crypto

// newSigner chooses and initializes a signature scheme
func newSigner(algo SigningAlgorithm) (signer, error) {
	return newNonRelicSigner(algo)
}

func init() {
	initNonRelic()
}
