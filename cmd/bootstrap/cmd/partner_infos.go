package cmd

import (
	"context"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/spf13/cobra"
)

var (
	flagOutputDir    string
	flagANAddress    string
	flagANNetworkKey string
	flagNetworkEnv   string

	getNodeInfoScript = []byte(`import FlowIDTableStaking from 0x9eca2b38b18b5dfe

	// This script gets all the info about a node and returns it
	
	pub fun main(nodeID: String): FlowIDTableStaking.NodeInfo {
		return FlowIDTableStaking.NodeInfo(nodeID: nodeID)
	}`)
)

const (
	// Index of each field in the cadence NodeInfo as it corresponds to cadence.Struct.Fields needed to build NodePubInfo struct,
	// fields not needed are left out.
	idField = iota
	roleField
	networkingAddressField
	networkingKeyField
	stakingKeyField
	tokensStakedField
	flowNodeAddrPart    = "nodes.onflow.org"
	defaultPartnerStake = 100
)

// NodePubInfo basic representation of node-pub-info.json data
type NodePubInfo struct {
	Role          string `json:"Role"`
	Address       string `json:"Address"`
	NodeID        string `json:"NodeID"`
	Stake         string `json:"Stake"`
	NetworkPubKey string `json:"NetworkPubKey"`
	StakingPubKey string `json:"StakingPubKey"`
}

// PartnerStakesInfo mapping of NodeID => weight of staking key
type PartnerStakesInfo map[flow.Identifier]uint64

// populatePartnerInfos represents the `populate-partner-infos` command which will read the proposed node
// table from the staking contract and for each identity in the proposed table generate a node-info-pub
// json file. It will also generate the partner-stakes json file.
var populatePartnerInfosCMD = &cobra.Command{
	Use:   "populate-partner-infos",
	Short: "Generates a node-info-pub-*.json for all proposed identities in staking contract and corresponding partner-stakes.json file.",
	Run:   populatePartnerInfosRun,
}

func init() {
	rootCmd.AddCommand(populatePartnerInfosCMD)

	populatePartnerInfosCMD.Flags().StringVar(&flagANAddress, "access-address", "", "the address of the access node used for client connections")
	populatePartnerInfosCMD.Flags().StringVar(&flagANNetworkKey, "access-network-key", "", "the network key of the access node used for client connections in hex string format")
	populatePartnerInfosCMD.Flags().StringVar(&flagNetworkEnv, "network", "mainnet", "the network string, expecting one of ( mainnet | testnet | localnet )")

	cmd.MarkFlagRequired(populatePartnerInfosCMD, "access-address")
}

// populatePartnerInfosRun generate node-pub-info file for each node in the proposed table and the partner stakes file, and prints the
// address and node ID of any flow nodes that were skipped.
func populatePartnerInfosRun(_ *cobra.Command, _ []string) {
	ctx := context.Background()

	env, err := common.EnvFromNetwork(flagNetworkEnv)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get environment for network (%s)", flagNetworkEnv)
	}

	flowClient := getFlowClient()

	proposedNodeIDS, err := executeGetProposedTableScript(ctx, env, flowClient)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get proposed table")
	}

	partnerStakes := make(PartnerStakesInfo)
	skippedNodes := 0
	numOfPartnerNodesByRole := map[flow.Role]int{
		flow.RoleCollection:   0,
		flow.RoleConsensus:    0,
		flow.RoleExecution:    0,
		flow.RoleVerification: 0,
		flow.RoleAccess:       0,
	}
	totalNumOfPartnerNodes := 0

	for _, id := range proposedNodeIDS.Values {
		info, err := executeGetNodeInfoScript(ctx, env, flowClient, id)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not get node info for node (%s)", id)
		}

		nodePubInfo, err := parseNodeInfo(info)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not node info")
		}

		if isFlowNode(nodePubInfo.Address) {
			skippedNodes++
			continue
		}

		writeNodePubInfoFile(nodePubInfo)
		partnerStakes[nodePubInfo.NodeID] = defaultPartnerStake
		numOfPartnerNodesByRole[nodePubInfo.Role]++
		totalNumOfPartnerNodes++
	}

	writePartnerStakesFile(partnerStakes)

	printNodeCounts(numOfPartnerNodesByRole, totalNumOfPartnerNodes, skippedNodes)
}

// getFlowClient will validate the flagANNetworkKey and return flow client
func getFlowClient() *client.Client {
	// default to insecure client connection
	insecureClient := true

	if flagANNetworkKey != "" {
		err := validateANNetworkKey(flagANNetworkKey)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to create flow client invalid access-network-key provided (%s)", flagANNetworkKey, err)
		}

		insecureClient = false
	}

	config, err := common.NewFlowClientConfig(flagANAddress, strings.TrimPrefix(flagANNetworkKey, "0x"), insecureClient)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get flow client config with address (%s) and network key (%s)", flagANAddress, flagANNetworkKey)
	}

	flowClient, err := common.FlowClient(config)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get flow client with address (%s) and network key (%s)", flagANAddress, flagANNetworkKey)
	}

	return flowClient
}

// executeGetProposedTableScript executes the get proposed table script
func executeGetProposedTableScript(ctx context.Context, env templates.Environment, flowClient *client.Client) (cadence.Array, error) {
	proposedTable, err := flowClient.ExecuteScriptAtLatestBlock(ctx, templates.GenerateReturnProposedTableScript(env), []cadence.Value{})
	if err != nil {
		return cadence.Array{}, fmt.Errorf("failed to execute the get proposed table script: %w", err)
	}

	return proposedTable.(cadence.Array), nil
}

// executeGetNodeInfoScript executes the get node info script
func executeGetNodeInfoScript(ctx context.Context, env templates.Environment, client *client.Client, nodeID cadence.Value) (cadence.Value, error) {
	info, err := client.ExecuteScriptAtLatestBlock(ctx, getNodeInfoScript, []cadence.Value{nodeID})
	if err != nil {
		return nil, fmt.Errorf("failed to execute the get node info script: %w", err)
	}

	return info, nil
}

// parseNodeInfo convert node info retrieved from cadence script
func parseNodeInfo(info cadence.Value) (*bootstrap.NodeInfoPub, error) {
	fields := info.(cadence.Struct).Fields
	nodeID, err := flow.HexStringToIdentifier(string(fields[idField].(cadence.String)))
	if err != nil {
		return nil, fmt.Errorf("failed to convert flow node ID from hex string to identifier (%s): %w", string(fields[idField].(cadence.String)), err)
	}

	b, err := hex.DecodeString(string(fields[networkingKeyField].(cadence.String)))
	if err != nil {
		return nil, fmt.Errorf("failed to decode network public key hex (%s): %w", string(fields[networkingKeyField].(cadence.String)), err)
	}
	networkPubKey, err := crypto.DecodePublicKey(crypto.ECDSAP256, b)
	if err != nil {
		return nil, fmt.Errorf("failed to decode network public key: %w", err)
	}

	b, err = hex.DecodeString(string(fields[stakingKeyField].(cadence.String)))
	if err != nil {
		return nil, fmt.Errorf("failed to decode staking public key hex (%s): %w", string(fields[stakingKeyField].(cadence.String)), err)
	}
	stakingPubKey, err := crypto.DecodePublicKey(crypto.BLSBLS12381, b)
	if err != nil {
		return nil, fmt.Errorf("failed to decode staking public key: %w", err)
	}

	return &bootstrap.NodeInfoPub{
		Role:          flow.Role(fields[roleField].(cadence.UInt8)),
		Address:       string(fields[networkingAddressField].(cadence.String)),
		NodeID:        nodeID,
		Stake:         uint64(fields[tokensStakedField].(cadence.UFix64)),
		NetworkPubKey: encodable.NetworkPubKey{PublicKey: networkPubKey},
		StakingPubKey: encodable.StakingPubKey{PublicKey: stakingPubKey},
	}, nil
}

// isFlowNode returns true if the address contains nodes.onflow.org
func isFlowNode(address string) bool {
	return strings.Contains(address, flowNodeAddrPart)
}

// validateANNetworkKey attempts to parse the network key provided for secure client connections
func validateANNetworkKey(key string) error {
	b, err := hex.DecodeString(strings.TrimPrefix(key, "0x"))
	if err != nil {
		return fmt.Errorf("failed to decode public key hex string: %w", err)
	}

	_, err = crypto.DecodePublicKey(crypto.ECDSAP256, b)
	if err != nil {
		return fmt.Errorf("failed to decode public key: %w", err)
	}

	return nil
}

// writeNodePubInfoFile writes the node-pub-info file
func writeNodePubInfoFile(info *bootstrap.NodeInfoPub) {
	fileOutputName := fmt.Sprintf(bootstrap.PathNodeInfoPub, info.NodeID)
	path := filepath.Join(flagOutputDir, fileOutputName)
	writeJSON(path, info)
}

// writePartnerStakesFile writes the partner stakes file
func writePartnerStakesFile(partnerStakes PartnerStakesInfo) {
	path := filepath.Join(flagOutputDir, bootstrap.PartnerStakesFileName)
	writeJSON(path, partnerStakes)
}

func printNodeCounts(numOfNodesByType map[flow.Role]int, totalNumOfPartnerNodes, skippedNodes int) {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Number of flow nodes skipped: %d\n", skippedNodes))
	builder.WriteString(fmt.Sprintf("Number of partner nodes: %d\n", totalNumOfPartnerNodes))
	builder.WriteString(fmt.Sprint("Number of partner nodes by role:"))
	for role, count := range numOfNodesByType {
		builder.WriteString(fmt.Sprintf("\t%s : %d", role, count))
	}
	builder.WriteString("\n")

	fmt.Println(builder.String())
}
