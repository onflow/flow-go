package unittest

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/model/flow"
)

// Used below with random service key
// privateKey := flow.AccountPrivateKey{
//	 PrivateKey: rootKey,
//	 SignAlgo:   crypto.ECDSAP256,
//	 HashAlgo:   hash.SHA2_256,
// }

const ServiceAccountPrivateKeyHex = "8ae3d0461cfed6d6f49bfc25fa899351c39d1bd21fdba8c87595b6c49bb4cc43"
const ServiceAccountPrivateKeySignAlgo = crypto.ECDSAP256
const ServiceAccountPrivateKeyHashAlgo = hash.SHA2_256

// Pre-calculated state commitment with root account with the above private key
const GenesisStateCommitmentHex = "3def1b719758aa82e62c670662db634233f0f95b3c5d5d98017a9fa1c843f9c5"

var GenesisStateCommitment flow.StateCommitment

var GenesisTokenSupply = func() cadence.UFix64 {
	// value, err := cadence.NewUFix64("10000000000.0") // 10 billion
	value, err := cadence.NewUFix64("1000000000.0") // 1 billion
	if err != nil {
		panic(fmt.Errorf("invalid genesis token supply: %w", err))
	}
	return value
}()

var ServiceAccountPrivateKey flow.AccountPrivateKey
var ServiceAccountPublicKey flow.AccountPublicKey

func init() {
	var err error
	GenesisStateCommitmentBytes, err := hex.DecodeString(GenesisStateCommitmentHex)
	if err != nil {
		panic("error while hex decoding hardcoded state commitment")
	}
	GenesisStateCommitment, err = flow.ToStateCommitment(GenesisStateCommitmentBytes)
	if err != nil {
		panic("genesis state commitment size is invalid")
	}

	serviceAccountPrivateKeyBytes, err := hex.DecodeString(ServiceAccountPrivateKeyHex)
	if err != nil {
		panic("error while hex decoding hardcoded root key")
	}

	ServiceAccountPrivateKey.SignAlgo = ServiceAccountPrivateKeySignAlgo
	ServiceAccountPrivateKey.HashAlgo = ServiceAccountPrivateKeyHashAlgo
	ServiceAccountPrivateKey.PrivateKey, err = crypto.DecodePrivateKey(
		ServiceAccountPrivateKey.SignAlgo, serviceAccountPrivateKeyBytes)
	if err != nil {
		panic("error while decoding hardcoded root key bytes")
	}

	// Cannot import virtual machine, due to circular dependency. Just use the value of
	// fvm.AccountKeyWeightThreshold here
	ServiceAccountPublicKey = ServiceAccountPrivateKey.PublicKey(1000)
}

// this is done by printing the state commitment in TestBootstrapLedger test with different chain ID
func GenesisStateCommitmentByChainID(chainID flow.ChainID) flow.StateCommitment {
	commitString := genesisCommitHexByChainID(chainID)
	bytes, err := hex.DecodeString(commitString)
	if err != nil {
		panic("error while hex decoding hardcoded state commitment")
	}
	commit, err := flow.ToStateCommitment(bytes)
	if err != nil {
		panic("genesis state commitment size is invalid")
	}
	return commit
}

func genesisCommitHexByChainID(chainID flow.ChainID) string {
	if chainID == flow.Mainnet {
		return GenesisStateCommitmentHex
	}
	if chainID == flow.Testnet {
		return "3a7cee57549e1ad8be6ef1f433fbdbb2a3a9b49c9f5deb5ffbcf050351b5b6b5"
	}
	if chainID == flow.Sandboxnet {
		return "e1c08b17f9e5896f03fe28dd37ca396c19b26628161506924fbf785834646ea1"
	}
	return "10bd01e39cc8d4e39b978d50f46bc9a63ee8b9830089a9ccda1ab13d78260ef2"
}
