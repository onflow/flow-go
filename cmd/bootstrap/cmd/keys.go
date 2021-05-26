package cmd

import (
	"fmt"

	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/crypto"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
)

func genNetworkAndStakingKeys() []model.NodeInfo {

	var nodeConfigs []model.NodeConfig
	readJSON(flagConfig, &nodeConfigs)
	nodes := len(nodeConfigs)
	log.Info().Msgf("read %v node configurations", nodes)

	validateAddressesUnique(nodeConfigs)
	log.Debug().Msg("all node addresses are unique")

	log.Debug().Msgf("will generate %v networking keys for nodes in config", nodes)
	networkKeys, err := run.GenerateNetworkingKeys(nodes, generateRandomSeeds(nodes))
	if err != nil {
		log.Fatal().Err(err).Msg("cannot generate networking keys")
	}
	log.Info().Msgf("generated %v networking keys for nodes in config", nodes)

	log.Debug().Msgf("will generate %v staking keys for nodes in config", nodes)
	stakingKeys, err := run.GenerateStakingKeys(nodes, generateRandomSeeds(nodes))
	if err != nil {
		log.Fatal().Err(err).Msg("cannot generate staking keys")
	}
	log.Info().Msgf("generated %v staking keys for nodes in config", nodes)

	internalNodes := make([]model.NodeInfo, 0, len(nodeConfigs))
	for i, nodeConfig := range nodeConfigs {
		log.Debug().Int("i", i).Str("address", nodeConfig.Address).Msg("assembling node information")

		nodeInfo := assembleNodeInfo(nodeConfig, networkKeys[i], stakingKeys[i])
		internalNodes = append(internalNodes, nodeInfo)

		// retrieve private representation of the node
		private, err := nodeInfo.Private()
		if err != nil {
			log.Fatal().Err(err).Msg("could not access private key for internal node")
		}

		writeJSON(fmt.Sprintf(model.PathNodeInfoPriv, nodeInfo.NodeID), private)
	}

	for _, nodeInfo := range internalNodes {
		// retrieve private representation of the node
		private, err := nodeInfo.Private()
		if err != nil {
			log.Fatal().Err(err).Msg("could not access private key for internal node")
		}

		writeJSON(fmt.Sprintf(model.PathNodeInfoPriv, nodeInfo.NodeID), private)
	}

	return internalNodes
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
		nodeConfig.Stake,
		networkKey,
		stakingKey,
	)

	return nodeInfo
}

func assembleNodeMachineAccountInfo(machineKey crypto.PrivateKey, accountAddress string) model.NodeMachineAccountInfo {
	log.Debug().Str("machineAccountPubKey", pubKeyToString(machineKey.PublicKey())).Msg("encoded public machine account key")
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
	log.Debug().Str("machineAccountPubKey", pubKeyToString(machineKey.PublicKey())).Msg("encoded public machine account key")
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
