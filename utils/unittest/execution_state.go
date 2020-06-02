package unittest

import (
	"encoding/hex"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Used below with random service key
// privateKey := flow.AccountPrivateKey{
//	 PrivateKey: rootKey,
//	 SignAlgo:   crypto.ECDSAP256,
//	 HashAlgo:   hash.SHA2_256,
// }

const ServiceAccountPrivateKeyHex = "e3a08ae3d0461cfed6d6f49bfc25fa899351c39d1bd21fdba8c87595b6c49bb4cc430201"
const ServiceAccountPrivateKeyHexSDK = "8ae3d0461cfed6d6f49bfc25fa899351c39d1bd21fdba8c87595b6c49bb4cc43"

// Pre-calculated state commitment with root account with the above private key
const GenesisStateCommitmentHex = "b0c3a818c6e1adee71c43ef3825636f377dd60e588afed450446f7b0a27ea397"

var GenesisStateCommitment flow.StateCommitment

const GenesisTokenSupply uint64 = 1_000_000_000_000_000

var ServiceAccountPrivateKey flow.AccountPrivateKey
var ServiceAccountPublicKey flow.AccountPublicKey

func init() {
	var err error
	GenesisStateCommitment, err = hex.DecodeString(GenesisStateCommitmentHex)
	if err != nil {
		panic("error while hex decoding hardcoded state commitment")
	}

	serviceAccountPrivateKeyBytes, err := hex.DecodeString(ServiceAccountPrivateKeyHex)
	if err != nil {
		panic("error while hex decoding hardcoded root key")
	}

	ServiceAccountPrivateKey, err = flow.DecodeAccountPrivateKey(serviceAccountPrivateKeyBytes)
	if err != nil {
		panic("error while decoding hardcoded root key bytes")
	}

	// Cannot import virtual machine, due to circular dependency. Just use the value of
	// virtualmachine.AccountKeyWeightThreshold here
	ServiceAccountPublicKey = ServiceAccountPrivateKey.PublicKey(1000)
}
