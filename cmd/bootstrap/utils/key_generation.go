package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	gohash "hash"
	"io"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/model/encodable"

	"golang.org/x/crypto/hkdf"

	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

// these constants are defined in X9.62 section 4.2 and 4.3
// see https://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.202.2977&rep=rep1&type=pdf
// they indicate if the conversion to/from a public key (point) in compressed form must involve an inversion of the ordinate coordinate
const X962_NO_INVERSION = uint8(0x02)
const X962_INVERSION = uint8(0x03)

func GenerateMachineAccountKey(seed []byte) (crypto.PrivateKey, error) {
	keys, err := GenerateKeys(crypto.ECDSAP256, 1, [][]byte{seed})
	if err != nil {
		return nil, err
	}
	return keys[0], nil
}

// GenerateSecretsDBEncryptionKey generates an encryption key for encrypting a
// Badger database.
func GenerateSecretsDBEncryptionKey() ([]byte, error) {
	// 32-byte key to use AES-256
	// https://pkg.go.dev/github.com/dgraph-io/badger/v2#Options.WithEncryptionKey
	const keyLen = 32

	key := make([]byte, keyLen)
	_, err := rand.Read(key)
	if err != nil {
		return nil, fmt.Errorf("could not generate key: %w", err)
	}
	return key, nil
}

// The unstaked nodes have special networking keys, in two aspects:
// - they use crypto.ECDSASecp256k1 keys, not crypto.ECDSAP256 keys,
// - they use only positive keys (in the sense that the elliptic curve point of their public key is positive)
//
// Thanks to various properties of the cryptographic algorithm and libp2p,
// this affords us to not have to maintain a table of flow.NodeID -> NetworkPublicKey
// for those numerous and ephemeral nodes.
// It incurs a one-bit security reduction, which is deemed acceptable.

// drawUnstakedKey draws a single positive ECDSASecp256k1 key, and returns an error otherwise.
func drawUnstakedKey(seed []byte) (crypto.PrivateKey, error) {
	key, err := crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
	if err != nil {
		// this should not happen
		return nil, err
	} else if key.PublicKey().EncodeCompressed()[0] == X962_INVERSION {
		// negative key -> unsuitable
		return nil, fmt.Errorf("Unsuitable negative key")
	}
	return key, nil
}

// GenerateUnstakedNetworkingKey draws ECDSASecp256k1 keys until finding a suitable one.
// though this will return fast, this is not constant-time and will leak ~1 bit of information through its runtime
func GenerateUnstakedNetworkingKey(seed []byte) (key crypto.PrivateKey, err error) {
	hkdf := hkdf.New(func() gohash.Hash { return sha256.New() }, seed, nil, []byte("unstaked network"))
	round_seed := make([]byte, len(seed))
	max_iterations := 20 // 1/(2^20) failure chance
	for i := 0; i < max_iterations; i++ {
		if _, err = io.ReadFull(hkdf, round_seed); err != nil {
			// the hkdf Reader should not fail
			panic(err)
		}
		if key, err = drawUnstakedKey(round_seed); err == nil {
			return
		}
	}
	return
}

func GenerateUnstakedNetworkingKeys(n int, seeds [][]byte) ([]crypto.PrivateKey, error) {
	if n != len(seeds) {
		return nil, fmt.Errorf("n needs to match the number of seeds (%v != %v)", n, len(seeds))
	}

	keys := make([]crypto.PrivateKey, n)

	var err error
	for i, seed := range seeds {
		if keys[i], err = GenerateUnstakedNetworkingKey(seed); err != nil {
			return nil, err
		}
	}

	return keys, nil
}

func GenerateNetworkingKey(seed []byte) (crypto.PrivateKey, error) {
	keys, err := GenerateKeys(crypto.ECDSAP256, 1, [][]byte{seed})
	if err != nil {
		return nil, err
	}
	return keys[0], nil
}

func GenerateNetworkingKeys(n int, seeds [][]byte) ([]crypto.PrivateKey, error) {
	return GenerateKeys(crypto.ECDSAP256, n, seeds)
}

func GenerateStakingKey(seed []byte) (crypto.PrivateKey, error) {
	keys, err := GenerateKeys(crypto.BLSBLS12381, 1, [][]byte{seed})
	if err != nil {
		return nil, err
	}
	return keys[0], nil
}

func GenerateStakingKeys(n int, seeds [][]byte) ([]crypto.PrivateKey, error) {
	return GenerateKeys(crypto.BLSBLS12381, n, seeds)
}

func GenerateKeys(algo crypto.SigningAlgorithm, n int, seeds [][]byte) ([]crypto.PrivateKey, error) {
	if n != len(seeds) {
		return nil, fmt.Errorf("n needs to match the number of seeds (%v != %v)", n, len(seeds))
	}

	keys := make([]crypto.PrivateKey, n)

	var err error
	for i, seed := range seeds {
		if keys[i], err = crypto.GeneratePrivateKey(algo, seed); err != nil {
			return nil, err
		}
	}

	return keys, nil
}

// WriteJSONFileFunc is a function which writes a file during bootstrapping. It
// accepts the path for the file (relative to the bootstrapping root directory)
// and the value to write. The function must marshal the value as JSON and write
// the result to the given path.
type WriteJSONFileFunc func(relativePath string, value interface{}) error

// WriteFileFunc is the same as WriteJSONFileFunc, but it writes the bytes directly
// rather than marshalling a structure to json.
type WriteFileFunc func(relativePath string, data []byte) error

// WriteMachineAccountFiles writes machine account key files for a set of nodeInfos.
// Assumes that machine accounts have been created using the default execution state
// bootstrapping. Further assumes that the order of nodeInfos is the same order that
// nodes were registered during execution state bootstrapping.
//
// Only applicable for transient test networks.
func WriteMachineAccountFiles(chainID flow.ChainID, nodeInfos []bootstrap.NodeInfo, write WriteJSONFileFunc) error {

	// ensure the chain ID is for a transient chain, where it is possible to
	// infer machine account addresses this way
	if !chainID.Transient() {
		return fmt.Errorf("cannot write default machine account files for non-transient network")
	}

	// write machine account key files for each node which has a machine account (LN/SN)
	//
	// for the machine account key, we keep track of the address index to map
	// the Flow address of the machine account to the key.
	addressIndex := uint64(4)
	for _, nodeInfo := range nodeInfos {

		// retrieve private representation of the node
		private, err := nodeInfo.Private()
		if err != nil {
			return err
		}

		// We use the network key for the machine account. Normally it would be
		// a separate key.

		// Accounts are generated in a known order during bootstrapping, and
		// account addresses are deterministic based on order for a given chain
		// configuration. During the bootstrapping we create 4 Flow accounts besides
		// the service account (index 0) so node accounts will start at index 5.
		//
		// All nodes have a staking account created for them, only collection and
		// consensus nodes have a second machine account created.
		//
		// The accounts are created in the same order defined by the identity list
		// provided to BootstrapProcedure, which is the same order as this iteration.
		if nodeInfo.Role == flow.RoleCollection || nodeInfo.Role == flow.RoleConsensus {
			// increment the address index to account for both the staking account
			// and the machine account.
			// now addressIndex points to the machine account address index
			addressIndex += 2
		} else {
			// increment the address index to account for the staking account
			// we don't need to persist anything related to the staking account
			addressIndex += 1
			continue
		}

		accountAddress, err := chainID.Chain().AddressAtIndex(addressIndex)
		if err != nil {
			return err
		}

		info := bootstrap.NodeMachineAccountInfo{
			Address:           accountAddress.HexWithPrefix(),
			EncodedPrivateKey: private.NetworkPrivKey.Encode(),
			KeyIndex:          0,
			SigningAlgorithm:  private.NetworkPrivKey.Algorithm(),
			HashAlgorithm:     sdkcrypto.SHA3_256,
		}

		path := fmt.Sprintf(bootstrap.PathNodeMachineAccountInfoPriv, nodeInfo.NodeID)
		err = write(path, info)
		if err != nil {
			return err
		}
	}

	return nil
}

func WriteMachineAccountFile(
	nodeID flow.Identifier,
	accountAddress sdk.Address,
	accountKey encodable.MachineAccountPrivKey,
	write WriteJSONFileFunc) error {

	info := bootstrap.NodeMachineAccountInfo{
		Address:           fmt.Sprintf("0x%s", accountAddress.Hex()),
		EncodedPrivateKey: accountKey.Encode(),
		KeyIndex:          0,
		SigningAlgorithm:  accountKey.Algorithm(),
		HashAlgorithm:     sdkcrypto.SHA3_256,
	}

	path := fmt.Sprintf(bootstrap.PathNodeMachineAccountInfoPriv, nodeID)
	err := write(path, info)
	if err != nil {
		return err
	}

	return nil
}

// WriteSecretsDBEncryptionKeyFiles writes secret db encryption keys to private
// node info directory.
func WriteSecretsDBEncryptionKeyFiles(nodeInfos []bootstrap.NodeInfo, write WriteFileFunc) error {

	for _, nodeInfo := range nodeInfos {

		// generate an encryption key for the node
		encryptionKey, err := GenerateSecretsDBEncryptionKey()
		if err != nil {
			return err
		}

		path := fmt.Sprintf(bootstrap.PathSecretsEncryptionKey, nodeInfo.NodeID)
		err = write(path, encryptionKey)
		if err != nil {
			return err
		}
	}
	return nil
}

// WriteStakingNetworkingKeyFiles writes staking and networking keys to private
// node info files.
func WriteStakingNetworkingKeyFiles(nodeInfos []bootstrap.NodeInfo, write WriteJSONFileFunc) error {

	for _, nodeInfo := range nodeInfos {

		// retrieve private representation of the node
		private, err := nodeInfo.Private()
		if err != nil {
			return err
		}

		path := fmt.Sprintf(bootstrap.PathNodeInfoPriv, nodeInfo.NodeID)
		err = write(path, private)
		if err != nil {
			return err
		}
	}

	return nil
}
