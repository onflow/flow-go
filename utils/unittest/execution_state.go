package unittest

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"

	"github.com/onflow/cadence"

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
const GenesisStateCommitmentHex = "9d7ff84d65cb92a7a2d3c4bc6d73d87790de377e96568019fb9489540340f3a0"

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
		return "7070cfefd29e6386684a84a009ff5500b7bb16b360fbed2e1a561aaed7d34fa8"
	}
	if chainID == flow.Sandboxnet {
		return "e1c08b17f9e5896f03fe28dd37ca396c19b26628161506924fbf785834646ea1"
	}
	return "e00f42cb8ef15a308662b8545bd04eee8640aad2e075bb94b5c67fcd0faba096"
}
