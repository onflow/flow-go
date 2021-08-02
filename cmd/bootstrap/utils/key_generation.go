package utils

import (
	"fmt"

	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

func GenerateMachineAccountKey(seed []byte) (crypto.PrivateKey, error) {
	keys, err := GenerateKeys(crypto.ECDSAP256, 1, [][]byte{seed})
	if err != nil {
		return nil, err
	}
	return keys[0], nil
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

// WriteMachineAccountFiles writes machine account key files for a set of nodeInfos.
// Assumes that machine accounts have been created using the default execution state
// bootstrapping. Further assumes that the order of nodeInfos is the same order that
// nodes were staked during execution state bootstrapping.
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
