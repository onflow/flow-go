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

// DecodeSingleSig decodes signature data into a cryptographic signature and a type as required by
// the consensus design.
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
