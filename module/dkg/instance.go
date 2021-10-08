package dkg

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// CanonicalInstanceID returns the canonical DKG instance ID for the given
// epoch on the given chain.
func CanonicalInstanceID(chainID flow.ChainID, epochCounter uint64) string {
	return fmt.Sprintf("dkg-%s-%d", chainID.String(), epochCounter)
}
