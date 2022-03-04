package cmd

import (
	"fmt"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow/order"

	"github.com/onflow/flow-go/crypto"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
)

// genNetworkAndStakingKeys generates network and staking keys for all nodes
// specified in the config and returns a list of NodeInfo objects containing
// these private keys.
func genNetworkAndStakingKeys() []model.NodeInfo {

	var nodeConfigs []model.NodeConfig
	readJSON(flagConfig, &nodeConfigs)
	nodes := len(nodeConfigs)
	log.Info().Msgf("read %v node configurations", nodes)

	validateAddressesUnique(nodeConfigs)
	log.Debug().Msg("all node addresses are unique")

	log.Debug().Msgf("will generate %v networking keys for nodes in config", nodes)
	networkKeys, err := utils.GenerateNetworkingKeys(nodes, GenerateRandomSeeds(nodes, crypto.KeyGenSeedMinLenECDSAP256))
	if err != nil {
		log.Fatal().Err(err).Msg("cannot generate networking keys")
	}
	log.Info().Msgf("generated %v networking keys for nodes in config", nodes)

	log.Debug().Msgf("will generate %v staking keys for nodes in config", nodes)
	stakingKeys, err := utils.GenerateStakingKeys(nodes, GenerateRandomSeeds(nodes, crypto.KeyGenSeedMinLenBLSBLS12381))
	if err != nil {
		log.Fatal().Err(err).Msg("cannot generate staking keys")
	}
	log.Info().Msgf("generated %v staking keys for nodes in config", nodes)

	internalNodes := make([]model.NodeInfo, 0, len(nodeConfigs))
	for i, nodeConfig := range nodeConfigs {
		log.Debug().Int("i", i).Str("address", nodeConfig.Address).Msg("assembling node information")
		nodeInfo := assembleNodeInfo(nodeConfig, networkKeys[i], stakingKeys[i])
		internalNodes = append(internalNodes, nodeInfo)
	}

	return model.Sort(internalNodes, order.Canonical)
}

func assembleNodeInfo(nodeConfig model.NodeConfig, networkKey, stakingKey crypto.PrivateKey) model.NodeInfo {
	var err error
	nodeID, found := getNameID()
	if !found {
		nodeID, err = flow.PublicKeyToID(stakingKey.PublicKey())
		if err != nil {
			log.Fatal().Err(err).Msg("cannot generate NodeID from PublicKey")
		}
	}

	log.Debug().
		Str("networkPubKey", pubKeyToString(networkKey.PublicKey())).
		Str("stakingPubKey", pubKeyToString(stakingKey.PublicKey())).
		Msg("encoded public staking and network keys")

	nodeInfo := model.NewPrivateNodeInfo(
		nodeID,
		nodeConfig.Role,
		nodeConfig.Address,
		nodeConfig.Weight,
		networkKey,
		stakingKey,
	)

	return nodeInfo
}

// AssembleNodeMachineAccountInfo exported wrapper for use in other projects
func AssembleNodeMachineAccountInfo(machineKey crypto.PrivateKey, accountAddress string) model.NodeMachineAccountInfo {
	return assembleNodeMachineAccountInfo(machineKey, accountAddress)
}

func assembleNodeMachineAccountInfo(machineKey crypto.PrivateKey, accountAddress string) model.NodeMachineAccountInfo {
	encAccountKey := encodedRuntimeAccountPubKey(machineKey)
	log.Debug().
		Str("machineAccountPubKey", fmt.Sprintf("%x", encAccountKey)).
		Msg("encoded public machine account key")
	machineNodeInfo := model.NodeMachineAccountInfo{
		EncodedPrivateKey: machineKey.Encode(),
		KeyIndex:          model.DefaultMachineAccountKeyIndex,
		SigningAlgorithm:  model.DefaultMachineAccountSignAlgo,
		HashAlgorithm:     model.DefaultMachineAccountHashAlgo,
		Address:           accountAddress,
	}
	return machineNodeInfo
}

// assembleNodeMachineAccountKey assembles the machine account info and logs
// the public key as it should be entered in Flow Port.
//
// We log the key as a flow.AccountPublicKey for input to Flow Port.
func assembleNodeMachineAccountKey(machineKey crypto.PrivateKey) model.NodeMachineAccountKey {
	encAccountKey := encodedRuntimeAccountPubKey(machineKey)
	log.Info().
		Str("machineAccountPubKey", fmt.Sprintf("%x", encAccountKey)).
		Msg("encoded machine account public key for entry to Flow Port")
	key := model.NodeMachineAccountKey{
		PrivateKey: encodable.MachineAccountPrivKey{PrivateKey: machineKey},
	}

	return key
}

func encodedRuntimeAccountPubKey(machineKey crypto.PrivateKey) []byte {
	accountKey := flow.AccountPublicKey{
		PublicKey: machineKey.PublicKey(),
		SignAlgo:  machineKey.Algorithm(),
		HashAlgo:  hash.SHA3_256,
		Weight:    1000,
	}
	encAccountKey, err := flow.EncodeRuntimeAccountPublicKey(accountKey)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to encode machine account public key")
	}
	return encAccountKey
}

func validateAddressesUnique(ns []model.NodeConfig) {
	lookup := make(map[string]struct{})
	for _, n := range ns {
		if _, ok := lookup[n.Address]; ok {
			log.Fatal().Str("address", n.Address).Msg("duplicate node address in config")
		}
	}
}
