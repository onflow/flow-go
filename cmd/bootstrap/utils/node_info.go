package utils

import (
	"fmt"
	"os"
	"path/filepath"

	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	io "github.com/onflow/flow-go/utils/io"
	"github.com/onflow/flow-go/utils/unittest"
)

// WritePartnerFiles writes the all partner public node info into `bootDir/partners/public-root-information/`
// also writes a map containing each of the nodes weights mapped by NodeID
func WritePartnerFiles(nodeInfos []model.NodeInfo, bootDir string) (string, string, error) {

	// convert to public nodeInfos and create a map from nodeID to weight
	nodePubInfos := make([]model.NodeInfoPub, len(nodeInfos))
	weights := make(map[flow.Identifier]uint64)
	for i, node := range nodeInfos {
		var err error
		nodePubInfos[i], err = node.Public()
		if err != nil {
			return "", "", fmt.Errorf("could not read public info: %w", err)
		}
		weights[node.NodeID] = node.Weight
	}

	// write node public infos to partner dir
	partnersDir := filepath.Join(bootDir, "partners")
	err := os.MkdirAll(filepath.Join(bootDir, partnersDir), os.ModePerm)
	if err != nil {
		return "", "", fmt.Errorf("could not create partner node info directory: %w", err)
	}

	// write each node info into partners dir
	for _, node := range nodePubInfos {
		nodePubInfosPath := filepath.Join(partnersDir, fmt.Sprintf(model.PathNodeInfoPub, node.NodeID.String()))
		err := io.WriteJSON(nodePubInfosPath, node)
		if err != nil {
			return "", "", fmt.Errorf("could not write partner node info: %w", err)
		}
	}

	// write partner weights
	weightsPath := filepath.Join(bootDir, model.FileNamePartnerWeights)
	err = io.WriteJSON(weightsPath, weights)
	if err != nil {
		return "", "", fmt.Errorf("could not write partner weights info: %w", err)
	}

	return filepath.Join(partnersDir, model.DirnamePublicBootstrap), weightsPath, nil
}

// WriteInternalFiles writes the internal private node info into `bootDir/private-root-information/`
// also writes a map containing each of the nodes weights mapped by the node's networking address
func WriteInternalFiles(nodeInfos []model.NodeInfo, bootDir string) (string, string, error) {

	// convert to private nodeInfos and node configuration map
	nodePrivInfos := make([]model.NodeInfoPriv, len(nodeInfos))
	configs := make([]model.NodeConfig, len(nodeInfos))
	for i, node := range nodeInfos {

		netPriv := unittest.NetworkingPrivKeyFixture()

		stakePriv := unittest.StakingPrivKeyFixture()

		nodePrivInfos[i] = model.NodeInfoPriv{
			Role:    node.Role,
			Address: node.Address,
			NodeID:  node.NodeID,
			NetworkPrivKey: encodable.NetworkPrivKey{
				PrivateKey: netPriv,
			},
			StakingPrivKey: encodable.StakingPrivKey{
				PrivateKey: stakePriv,
			},
		}

		configs[i] = model.NodeConfig{
			Role:    node.Role,
			Address: node.Address,
			Weight:  node.Weight,
		}
	}

	// write config
	configPath := filepath.Join(bootDir, "node-internal-infos.pub.json")
	err := io.WriteJSON(configPath, configs)
	if err != nil {
		return "", "", fmt.Errorf("could not write internal node configuration: %w", err)
	}

	// write node private infos to internal priv dir
	for _, node := range nodePrivInfos {
		internalPrivPath := fmt.Sprintf(model.PathNodeInfoPriv, node.NodeID)
		err = io.WriteJSON(filepath.Join(bootDir, internalPrivPath), node)
		if err != nil {
			return "", "", fmt.Errorf("could not write internal node info: %w", err)
		}
	}

	return bootDir, configPath, nil
}

func GenerateNodeInfos(consensus, collection, execution, verification, access int) []model.NodeInfo {

	nodes := make([]model.NodeInfo, 0)

	// CONSENSUS
	consensusNodes := unittest.NodeInfosFixture(consensus,
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithInitialWeight(flow.DefaultInitialWeight),
	)
	nodes = append(nodes, consensusNodes...)

	// COLLECTION
	collectionNodes := unittest.NodeInfosFixture(collection,
		unittest.WithRole(flow.RoleCollection),
		unittest.WithInitialWeight(flow.DefaultInitialWeight),
	)
	nodes = append(nodes, collectionNodes...)

	// EXECUTION
	executionNodes := unittest.NodeInfosFixture(execution,
		unittest.WithRole(flow.RoleExecution),
		unittest.WithInitialWeight(flow.DefaultInitialWeight),
	)
	nodes = append(nodes, executionNodes...)

	// VERIFICATION
	verificationNodes := unittest.NodeInfosFixture(verification,
		unittest.WithRole(flow.RoleVerification),
		unittest.WithInitialWeight(flow.DefaultInitialWeight),
	)
	nodes = append(nodes, verificationNodes...)

	// ACCESS
	accessNodes := unittest.NodeInfosFixture(access,
		unittest.WithRole(flow.RoleAccess),
		unittest.WithInitialWeight(flow.DefaultInitialWeight),
	)
	nodes = append(nodes, accessNodes...)

	return nodes
}
