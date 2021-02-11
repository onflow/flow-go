package unittest

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/model/flow"
)

// Used below with random service key
// privateKey := flow.AccountPrivateKey{
//	 PrivateKey: rootKey,
//	 SignAlgo:   crypto.ECDSAP256,
//	 HashAlgo:   hash.SHA2_256,
// }

const ServiceAccountPrivateKeyHex = "e3a08ae3d0461cfed6d6f49bfc25fa899351c39d1bd21fdba8c87595b6c49bb4cc430201"

// Pre-calculated state commitment with root account with the above private key
const GenesisStateCommitmentHex = "1abd0d7da12fbae71913d0280d2b100acf73cf3ac4ae37be8a65384a521219d8"

var GenesisStateCommitment flow.StateCommitment

var GenesisTokenSupply = func() cadence.UFix64 {
	value, err := cadence.NewUFix64("10000000.00000000")
	if err != nil {
		panic(fmt.Errorf("invalid genesis token supply: %w", err))
	}
	return value
}()

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
	// fvm.AccountKeyWeightThreshold here
	ServiceAccountPublicKey = ServiceAccountPrivateKey.PublicKey(1000)
}
