package signature

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
)

// RandomBeaconReconstructorImpl wraps the thresholdSigner, and translates the signer identity
// into signer index
type RandomBeaconReconstructorImpl struct {
	identity2SignerIndex map[flow.Identifier]int  // lookup signer index by identity
	thresholdSigner      hotstuff.ThresholdSigner // a stateful signer object for this block
}
