package signature

import (
	"fmt"

	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/model/encoding"
)

const SigLen = crypto.SignatureLenBLSBLS12381

// EncodeSingleSig encodes a single signature into signature data as required by the consensus design.
func EncodeSingleSig(sigType encoding.SigType, sig crypto.Signature) []byte {
	t := byte(sigType)
	encoded := make([]byte, 0, len(sig)+1)
	encoded = append(encoded, t)
	encoded = append(encoded, sig[:]...)
	return encoded
}

// DecodeSingleSig decodes the signature data into a cryptographic signature and a type as required by
// the consensus design. Cryptographic validity of signatures is _not_ checked.
// It returns:
//   - 0, nil, ErrInvalidSignatureFormat if the sig type is invalid (covers nil or empty sigData)
//   - sigType, signature, nil if the sig type is valid and the decoding is done successfully.
func DecodeSingleSig(sigData []byte) (encoding.SigType, crypto.Signature, error) {
	if len(sigData) == 0 {
		return 0, nil, fmt.Errorf("empty sig data: %w", ErrInvalidSignatureFormat)
	}

	sigType := encoding.SigType(sigData[0])
	if !sigType.Valid() {
		return 0, nil, fmt.Errorf("invalid sig type %v: %w", sigType, ErrInvalidSignatureFormat)
	}

	sig := crypto.Signature(sigData[1:])
	return sigType, sig, nil
}

// TODO: to be removed in V3, replace by packer's pack method
// EncodeDoubleSig encodes both the staking signature and random beacon signature
// into one sigData.
func EncodeDoubleSig(stakingSig crypto.Signature, beaconSig crypto.Signature) []byte {
	encoded := make([]byte, 0, len(stakingSig)+len(beaconSig))
	encoded = append(encoded, stakingSig...)
	encoded = append(encoded, beaconSig...)
	return encoded
}

// TODO: to be removed in V3, replace by packer's unpack method
// DecodeDoubleSig decodes the signature data into a staking signature and an optional
// random beacon signature. The decoding assumes BLS with BLS12-381 is used.
// Cryptographic validity of signatures is _not_ checked.
// Decomposition of the sigData is purely done based on length.
// It returns:
//   - staking signature, random beacon signature, nil:
//     if sigData is twice the size of a BLS signature bytes long, we use the leading half as staking signature
//     and the tailing half random beacon sig
//   - staking signature, nil, nil:
//     if sigData is the size of a BLS signature, we interpret sigData entirely as staking signature
//   - nil, nil, ErrInvalidSignatureFormat if the sig type is invalid (covers nil or empty sigData)
func DecodeDoubleSig(sigData []byte) (crypto.Signature, crypto.Signature, error) {
	sigLength := len(sigData)
	switch sigLength {
	case SigLen:
		return sigData, nil, nil
	case 2 * SigLen:
		return sigData[:SigLen], sigData[SigLen:], nil
	}

	return nil, nil, fmt.Errorf("invalid sig data length %d: %w", sigLength, ErrInvalidSignatureFormat)
}
