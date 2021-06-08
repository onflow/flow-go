// +build !relic

package crypto

import "github.com/onflow/flow-go/crypto/hash"

// Temporary stubs for BLS implementation. This is used only when the crypto library is built without the relic tag.
// This is currently just the emulator.

const blsNotSupported = "BLSKMAC not supported when flow-go is build without relic"

func NewBLSKMAC(_ string) hash.Hasher {
	panic(blsNotSupported)
}

func BLSGeneratePOP(_ PrivateKey) (Signature, error) {
	panic(blsNotSupported)
}

func BLSVerifyPOP(_ PublicKey, _ Signature) (bool, error) {
	panic(blsNotSupported)
}

func AggregateBLSSignatures(_ []Signature) (Signature, error) {
	panic(blsNotSupported)
}

func AggregateBLSPrivateKeys(_ []PrivateKey) (PrivateKey, error) {
	panic(blsNotSupported)
}

func AggregateBLSPublicKeys(_ []PublicKey) (PublicKey, error) {
	panic(blsNotSupported)
}

func NeutralBLSPublicKey() PublicKey {
	panic(blsNotSupported)
}

func RemoveBLSPublicKeys(_ PublicKey, _ []PublicKey) (PublicKey, error) {
	panic(blsNotSupported)
}

func VerifyBLSSignatureManyMessages(_ []PublicKey, _ Signature,
	_ [][]byte, _ []hash.Hasher) (bool, error) {
	panic(blsNotSupported)
}

func BatchVerifyBLSSignaturesOneMessage(_ []PublicKey, _ []Signature,
	_ []byte, _ hash.Hasher) ([]bool, error) {
	panic(blsNotSupported)
}
