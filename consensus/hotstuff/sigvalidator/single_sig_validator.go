package sigvalidator

import "github.com/onflow/flow-go/model/flow"

// SingleSigValidator implements the SigValidator interface and owns the knowledge of how to
// decode the signer and signature fields. It converts the vote/block into general message and
// signatures for the underneath stateful verifier to do pure signature verify.
// It's used by collection cluster, each signer only signs one staking sig with staking key.
// It which doesn't use threshold signature, therefore the signature validation is stateless.
type SingleSigValidator struct {
	identity2SignerIndex map[flow.Identifier]int // lookup signer index by identity
}
