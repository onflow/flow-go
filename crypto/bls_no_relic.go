//go:build !relic
// +build !relic

package crypto

import (
	"github.com/onflow/flow-go/crypto/hash"
)

// The functions below are the non-Relic versions of the public APIs
// requiring the Relic library.
// All BLS functionalities in the package require the Relic dependency,
// and therefore the "relic" build tag.
// Building without the "relic" tag is successful, but and calling one of the
// BLS functions results in a runtime panic. This allows projects depending on the
// crypto library to build successfully with or without the "relic" tag.

const relic_panic = "function is not supported when building without \"relic\" Go build tag"

const (
	SignatureLenBLSBLS12381     = 48
	KeyGenSeedMinLenBLSBLS12381 = 48
)

// bls.go functions
func NewExpandMsgXOFKMAC128(tag string) hash.Hasher {
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

func IdentityBLSPublicKey() PublicKey {
	panic(relic_panic)
}

func IsBLSAggregateEmptyListError(err error) bool {
	panic(relic_panic)
}

func IsInvalidSignatureError(err error) bool {
	panic(relic_panic)
}

func IsNotBLSKeyError(err error) bool {
	panic(relic_panic)
}

func IsBLSSignatureIdentity(s Signature) bool {
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

func SPOCKProve(sk PrivateKey, data []byte, kmac hash.Hasher) (Signature, error) {
	panic(relic_panic)
}

func SPOCKVerifyAgainstData(pk PublicKey, proof Signature, data []byte, kmac hash.Hasher) (bool, error) {
	panic(relic_panic)
}

func SPOCKVerify(pk1 PublicKey, proof1 Signature, pk2 PublicKey, proof2 Signature) (bool, error) {
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
) (ThresholdSignatureParticipant, error) {
	panic(relic_panic)
}

func NewBLSThresholdSignatureInspector(
	groupPublicKey PublicKey,
	sharePublicKeys []PublicKey,
	threshold int,
	message []byte,
	dsTag string,
) (ThresholdSignatureInspector, error) {
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
	processor DKGProcessor, dealerIndex int) (DKGState, error) {
	panic(relic_panic)
}

func NewFeldmanVSSQual(size int, threshold int, myIndex int,
	processor DKGProcessor, dealerIndex int) (DKGState, error) {
	panic(relic_panic)
}

func NewJointFeldman(size int, threshold int, myIndex int,
	processor DKGProcessor) (DKGState, error) {
	panic(relic_panic)
}
