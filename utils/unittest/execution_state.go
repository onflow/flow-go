package unittest

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/fvm/cadence_vm"
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
var GenesisStateCommitmentHex = ifCompile(
	"2aec1528d538f64d05d0fd7e1726f9bb5fd46ec904d2df87212c308235affc94",
	"002ec8b7ddc16a5f057539433ab9e4190806a231a0ca00a1c666745c44104121",
)

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

func ifCompile[T any](a, b T) T {
	if cadence_vm.DefaultEnabled {
		return a
	}
	return b
}

func genesisCommitHexByChainID(chainID flow.ChainID) string {
	if chainID == flow.Mainnet {
		return GenesisStateCommitmentHex
	}
	if chainID == flow.Testnet {
		return ifCompile(
			"85a5558c2a25a655d68fdcb29d4fa4dc457d86425da11d9ae7a1edd2dc859e2a",
			"475d23a46aed725c9663bf77e8b41ab569d57bd52529840b38e730d031d662bf",
		)
	}
	if chainID == flow.Sandboxnet {
		return "e1c08b17f9e5896f03fe28dd37ca396c19b26628161506924fbf785834646ea1"
	}
	return ifCompile(
		"848de583931aaeb3eaf2cf144c5ad53df0d4704a7798205dc3e752e1e63d23a1",
		"6a3bc6034668d0a8ce4e95e61482d9c729d8075b96e378d4583cf4398b3f3863",
	)
}
