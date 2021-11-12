package signature

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
	msig "github.com/onflow/flow-go/module/signature"
)

// EncodeSingleSig encodes a single signature into signature data as required by the consensus design.
func EncodeSingleSig(sigType hotstuff.SigType, sig crypto.Signature) []byte {
	t := byte(sigType)
	encoded := make([]byte, 0, len(sig)+1)
	encoded = append(encoded, t)
	encoded = append(encoded, sig[:]...)
	return encoded
}

// DecodeSingleSig decodes the signature data into a cryptographic signature and a type as required by
// the consensus design. Cryptographic validity of signatures is _not_ checked.
// It returns:
//  - 0, nil, ErrInvalidFormat if the sig type is invalid (covers nil or empty sigData)
//  - sigType, signature, nil if the sig type is valid and the decoding is done successfully.
func DecodeSingleSig(sigData []byte) (hotstuff.SigType, crypto.Signature, error) {
	if len(sigData) == 0 {
		return 0, nil, fmt.Errorf("empty sig data: %w", msig.ErrInvalidFormat)
	}

	sigType := hotstuff.SigType(sigData[0])
	if !sigType.Valid() {
		return 0, nil, fmt.Errorf("invalid sig type %v: %w", sigType, msig.ErrInvalidFormat)
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
// random beacon signature. Cryptographic validity of signatures is _not_ checked.
// Decomposition of the sigData is purely done based on length.
// It returns:
//  - staking signature, random beacon signature, nil:
//    if sigData is 96 bytes long, we use the leading 48 bytes as staking signature
//    and the tailing 48 bytes as random beacon sig
//  - staking signature, nil, nil:
//    if sigData is 48 bytes long, we interpret sigData entirely as staking signature
//  - nil, nil, ErrInvalidFormat if the sig type is invalid (covers nil or empty sigData)
func DecodeDoubleSig(sigData []byte) (crypto.Signature, crypto.Signature, error) {
	sigLength := len(sigData)
	switch sigLength {
	case 48:
		return sigData, nil, nil
	case 96:
		return sigData[:48], sigData[48:], nil
	}

	return nil, nil, fmt.Errorf("invalid sig data length %d: %w", sigLength, msig.ErrInvalidFormat)
}
