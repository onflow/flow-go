//go:build !relic
// +build !relic

package crypto

func SPOCKVerify(pk1 PublicKey, proof1 Signature, pk2 PublicKey, proof2 Signature) (bool, error) {
	panic(relic_panic)
}
