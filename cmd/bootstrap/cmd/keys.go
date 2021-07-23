package cmd

import (
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
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
	utils.ReadJSON(flagConfig, &nodeConfigs)
	nodes := len(nodeConfigs)
	log.Info().Msgf("read %v node configurations", nodes)

	validateAddressesUnique(nodeConfigs)
	log.Debug().Msg("all node addresses are unique")

	log.Debug().Msgf("will generate %v networking keys for nodes in config", nodes)
	networkKeys, err := utils.GenerateNetworkingKeys(nodes, utils.GenerateRandomSeeds(nodes))
	if err != nil {
		log.Fatal().Err(err).Msg("cannot generate networking keys")
	}
	log.Info().Msgf("generated %v networking keys for nodes in config", nodes)

	log.Debug().Msgf("will generate %v staking keys for nodes in config", nodes)
	stakingKeys, err := utils.GenerateStakingKeys(nodes, utils.GenerateRandomSeeds(nodes))
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
		Str("networkPubKey", utils.PubKeyToString(networkKey.PublicKey())).
		Str("stakingPubKey", utils.PubKeyToString(stakingKey.PublicKey())).
		Msg("encoded public staking and network keys")

	nodeInfo := model.NewPrivateNodeInfo(
		nodeID,
		nodeConfig.Role,
		nodeConfig.Address,
		nodeConfig.Stake,
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
	log.Debug().Str("machineAccountPubKey", utils.PubKeyToString(machineKey.PublicKey())).Msg("encoded public machine account key")
	machineNodeInfo := model.NodeMachineAccountInfo{
		EncodedPrivateKey: machineKey.Encode(),
		KeyIndex:          0,
		SigningAlgorithm:  sdkcrypto.ECDSA_P256,
		HashAlgorithm:     sdkcrypto.SHA3_256,
		Address:           accountAddress,
	}
	return machineNodeInfo
}

func assembleNodeMachineAccountKey(machineKey crypto.PrivateKey) model.NodeMachineAccountKey {
	log.Debug().Str("machineAccountPubKey", utils.PubKeyToString(machineKey.PublicKey())).Msg("encoded public machine account key")
	key := model.NodeMachineAccountKey{
		PrivateKey: encodable.MachineAccountPrivKey{PrivateKey: machineKey},
	}
	return key
}

func validateAddressesUnique(ns []model.NodeConfig) {
	lookup := make(map[string]struct{})
	for _, n := range ns {
		if _, ok := lookup[n.Address]; ok {
			log.Fatal().Str("address", n.Address).Msg("duplicate node address in config")
		}
	}
}
