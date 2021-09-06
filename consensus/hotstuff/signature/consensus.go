package signature

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
	msig "github.com/onflow/flow-go/module/signature"
)

func EncodeSingleSig(sigType hotstuff.SigType, sig crypto.Signature) []byte {
	enc := make([]byte, 0, 1+len(sig))
	enc = append(enc, sigType)
	enc = append(enc, sig...)
	return enc
}

// DecodeSingleSig decodes byte representation into signature type and the
// raw signature. Error returns (expected during normal operations):
//  * signature.ErrInvalidFormat
//    if the sigData does not comply with the expected format
func DecodeSingleSig(sigData []byte) (hotstuff.SigType, crypto.Signature, error) {
	if len(sigData) <= 1 {
		return 0, nil, fmt.Errorf("expecting at least 2bytes of signature data but got %d: %w", len(sigData), msig.ErrInvalidFormat)
	}
	return sigData[0], sigData[1:], nil
}
