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

// DecodeDoubleSig decodes the signature data into a staking signature and an optional
// random beacon signature. The decoding assumes BLS with BLS12-381 is used.
// Cryptographic validity of signatures is _not_ checked.
// Decomposition of the sigData is purely done based on length.
// It returns:
//  - staking signature, random beacon signature, nil:
//    if sigData is twice the size of a BLS signature bytes long, we use the leading half as staking signature
//    and the tailing half random beacon sig
//  - staking signature, nil, nil:
//    if sigData is the size of a BLS signature, we interpret sigData entirely as staking signature
//  - nil, nil, ErrInvalidFormat if the sig type is invalid (covers nil or empty sigData)
func DecodeDoubleSig(sigData []byte) (crypto.Signature, crypto.Signature, error) {
	sigLength := len(sigData)
	blsSigLen := crypto.SignatureLenBLSBLS12381
	switch sigLength {
	case blsSigLen:
		return sigData, nil, nil
	case 2 * blsSigLen:
		return sigData[:blsSigLen], sigData[blsSigLen:], nil
	}

	return nil, nil, fmt.Errorf("invalid sig data length %d: %w", sigLength, msig.ErrInvalidFormat)
}
