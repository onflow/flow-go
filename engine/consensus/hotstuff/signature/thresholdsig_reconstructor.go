// +build relic

package signature

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// DKGPublicData is the public data for DKG participants who generated their key shares
type DKGPublicData struct {
	GroupPubKey        crypto.PublicKey         // the group public key
	SignerIndexMapping map[crypto.PublicKey]int // the mapping from public key to signer index
}

// LookupIndex looks up the signer indices for given slice of public keys
func (d *DKGPublicData) LookupIndex(signerKeys []crypto.PublicKey) (bool, []int) {
	signerIndexes := make([]int, 0, len(signerKeys))
	for _, signerKey := range signerKeys {
		signerIndex, found := d.SignerIndexMapping[signerKey]
		if !found {
			return false, nil
		}
		signerIndexes = append(signerIndexes, signerIndex)
	}
	return true, signerIndexes
}

// Size returns the total number of participants
func (d DKGPublicData) Size() int {
	return len(d.SignerIndexMapping)
}
