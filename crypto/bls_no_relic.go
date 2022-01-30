//go:build !relic
// +build !relic

package crypto

import (
	"github.com/onflow/flow-go/crypto"
)

// Wrappers around the flow-go/crypto package.
// These functions are needed because the emulator requires the fvm package as
// a dependency, while the emulator is not built using the "relic" build tag.
// These functions insure the emulator is still compilable without the "relic"
// build tag. None of these functions is run by the emulator.

// The functions below are the non-Relic versions of the public APIs
// requiring the Relic library.
// All BLS functionalties in the package require the Relic dependency,
// and therefore the "relic" build tag.
// Building without the "relic" tag is successful, but and calling one of the
// BLS functions results in a runtime panic. This allows projects depending on the
// crypto library to build successfully with or without the "relic" tag.

const relic_panic = "not supported when built without relic build tag"

// bls.go functions
func NewBLSKMAC(tag string) hash.Hasher {
	panic(relic_panic)
}

func BLSInvalidSignature() Signature {
	panic(relic_panic)
}

// bls_multisig.go functions
func BLSGeneratePOP(sk PrivateKey) (Signature, error) {
	panic(relic_panic)
}

func BLSVerifyPOP(pk PublicKey, s Signature) (bool, error) {
	panic(relic_panic)
}

func AggregateBLSSignatures(sigs []Signature) (Signature, error) {
	panic(relic_panic)
}

func AggregateBLSPrivateKeys(keys []PrivateKey) (PrivateKey, error) {
	panic(relic_panic)
}

func AggregateBLSPublicKeys(keys []PublicKey) (PublicKey, error) {
	panic(relic_panic)
}

func NeutralBLSPublicKey() PublicKey {
	panic(relic_panic)
}

func RemoveBLSPublicKeys(aggKey PublicKey, keysToRemove []PublicKey) (PublicKey, error) {
	panic(relic_panic)
}

func VerifyBLSSignatureOneMessage(pks []PublicKey, s Signature,
	message []byte, kmac hash.Hasher) (bool, error) {
	panic(relic_panic)
}

func VerifyBLSSignatureManyMessages(pks []PublicKey, s Signature,
	messages [][]byte, kmac []hash.Hasher) (bool, error) {
	panic(relic_panic)
}

func BatchVerifyBLSSignaturesOneMessage(pks []PublicKey, sigs []Signature,
	message []byte, kmac hash.Hasher) ([]bool, error) {
	panic(relic_panic)
}

// bls_threshold.go functions
func NewBLSThresholdSignatureParticipant(
	groupPublicKey PublicKey,
	sharePublicKeys []PublicKey,
	threshold int,
	myIndex int,
	myPrivateKey PrivateKey,
	message []byte,
	dsTag string,
) (*blsThresholdSignatureParticipant, error) {
	panic(relic_panic)
}

func NewBLSThresholdSignatureInspector(
	groupPublicKey PublicKey,
	sharePublicKeys []PublicKey,
	threshold int,
	message []byte,
	dsTag string,
) (*blsThresholdSignatureInspector, error) {
	panic(relic_panic)
}

func BLSReconstructThresholdSignature(size int, threshold int,
	shares []Signature, signers []int) (Signature, error) {
	panic(relic_panic)
}

func EnoughShares(threshold int, sharesNumber int) (bool, error) {
	panic(relic_panic)
}

func BLSThresholdKeyGen(size int, threshold int, seed []byte) ([]PrivateKey,
	[]PublicKey, PublicKey, error) {
	panic(relic_panic)
}

// dkg.go functions
func NewFeldmanVSS(size int, threshold int, myIndex int,
	processor DKGProcessor, leaderIndex int) (DKGState, error) {
	panic(relic_panic)
}

func NewFeldmanVSSQual(size int, threshold int, myIndex int,
	processor DKGProcessor, leaderIndex int) (DKGState, error) {
	panic(relic_panic)
}

func NewJointFeldman(size int, threshold int, myIndex int,
	processor DKGProcessor) (DKGState, error) {
	panic(relic_panic)
}
