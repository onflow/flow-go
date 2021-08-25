package signature

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
)

func EncodeSingleSig(sigType hotstuff.SigType, sig crypto.Signature) []byte {
	panic("to be implemented")
}

func DecodeSingleSig(sigData []byte) (hotstuff.SigType, crypto.Signature, error) {
	panic("to be implemented")
}
