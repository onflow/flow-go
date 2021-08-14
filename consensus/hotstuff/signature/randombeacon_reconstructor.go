package signature

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
)

// RandomBeaconReconstructorImpl wraps the thresholdSigner, and translates the signer identity
// into signer index
type RandomBeaconReconstructorImpl struct {
	identity2SignerIndex map[flow.Identifier]int     // lookup signer index by identity
	randomBeaconSigner   hotstuff.RandomBeaconSigner // a stateful object for this block. It's used for both storing all sig shares and producing the node's own share by signing the block
}
