package cmd

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/onflow/cadence"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
)

const (
	// Index of each field in the cadence NodeInfo as it corresponds to cadence.Struct.Fields
	// fields not needed are left out.
	idField = iota
	roleField
	networkingAddressField
	networkingKeyField
	stakingKeyField
)

const (
	flowNodeAddrPart = "nodes.onflow.org"
)

var (
	flagANAddress    string
	flagANNetworkKey string
	flagNetworkEnv   string
)

// populatePartnerInfos represents the `populate-partner-infos` command which will read the proposed node
// table from the staking contract and for each identity in the proposed table generate a node-info-pub
// json file. It will also generate the partner-weights json file.
var populatePartnerInfosCMD = &cobra.Command{
	Use:   "populate-partner-infos",
	Short: "Generates a node-info-pub-*.json for all proposed identities in staking contract and corresponding partner-weights.json file.",
	Run:   populatePartnerInfosRun,
}

func init() {
	rootCmd.AddCommand(populatePartnerInfosCMD)

	populatePartnerInfosCMD.Flags().StringVar(&flagANAddress, "access-address", "", "the address of the access node used for client connections")
	populatePartnerInfosCMD.Flags().StringVar(&flagANNetworkKey, "access-network-key", "", "the network key of the access node used for client connections in hex string format")
	populatePartnerInfosCMD.Flags().StringVar(&flagNetworkEnv, "network", "mainnet", "the network string, expecting one of ( mainnet | testnet | emulator )")

	cmd.MarkFlagRequired(populatePartnerInfosCMD, "access-address")
}

// populatePartnerInfosRun generate node-pub-info file for each node in the proposed table and the partner weights file, and prints the
// address and node ID of any flow nodes that were skipped.
func populatePartnerInfosRun(_ *cobra.Command, _ []string) {
	ctx := context.Background()

	flowClient := getFlowClient()

	partnerWeights := make(PartnerWeights)
	skippedNodes := 0
	numOfPartnerNodesByRole := map[flow.Role]int{
		flow.RoleCollection:   0,
		flow.RoleConsensus:    0,
		flow.RoleExecution:    0,
		flow.RoleVerification: 0,
		flow.RoleAccess:       0,
	}
	totalNumOfPartnerNodes := 0

	nodeInfos, err := executeGetProposedNodesInfosScript(ctx, flowClient)
	if err != nil {
		log.Fatal().Err(err).Msg("could not get node info for nodes in the proposed table")
	}

	for _, info := range nodeInfos.(cadence.Array).Values {
		nodePubInfo, err := parseNodeInfo(info)
		if err != nil {
			log.Fatal().Err(err).Msg("could not parse node info from cadence script")
		}

		if isFlowNode(nodePubInfo.Address) {
			skippedNodes++
			continue
		}

		writeNodePubInfoFile(nodePubInfo)
		partnerWeights[nodePubInfo.NodeID] = flow.DefaultInitialWeight
		numOfPartnerNodesByRole[nodePubInfo.Role]++
		totalNumOfPartnerNodes++
	}

	writePartnerWeightsFile(partnerWeights)

	printNodeCounts(numOfPartnerNodesByRole, totalNumOfPartnerNodes, skippedNodes)
}

// getFlowClient will validate the flagANNetworkKey and return flow client
func getFlowClient() *client.Client {
	// default to insecure client connection
	insecureClient := true

	if flagANNetworkKey != "" {
		err := validateANNetworkKey(flagANNetworkKey)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to create flow client invalid access-network-key provided (%s)", flagANNetworkKey)
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

// executeGetProposedNodesInfosScript executes the get node info for each ID in the proposed table
func executeGetProposedNodesInfosScript(ctx context.Context, client *client.Client) (cadence.Value, error) {
	script, err := common.GetNodeInfoForProposedNodesScript(flagNetworkEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to get cadence script: %w", err)
	}

	infos, err := client.ExecuteScriptAtLatestBlock(ctx, script, []cadence.Value{})
	if err != nil {
		return nil, fmt.Errorf("failed to execute the get node info script: %w", err)
	}

	return infos, nil
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
		Weight:        flow.DefaultInitialWeight,
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
	fileOutputPath := fmt.Sprintf(bootstrap.PathNodeInfoPub, info.NodeID)
	writeJSON(fileOutputPath, info)
}

// writePartnerWeightsFile writes the partner weights file
func writePartnerWeightsFile(partnerWeights PartnerWeights) {
	writeJSON(bootstrap.FileNamePartnerWeights, partnerWeights)
}

func printNodeCounts(numOfNodesByType map[flow.Role]int, totalNumOfPartnerNodes, skippedNodes int) {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Total number of flow nodes (skipped): %d\n", skippedNodes))
	builder.WriteString(fmt.Sprintf("Total number of partner nodes: %d\n", totalNumOfPartnerNodes))
	builder.WriteString("Number of partner nodes by role:")
	for role, count := range numOfNodesByType {
		builder.WriteString(fmt.Sprintf("\t%s : %d", role, count))
	}
	builder.WriteString("\n")

	fmt.Println(builder.String())
}
