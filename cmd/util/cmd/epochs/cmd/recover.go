package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	epochcmdutil "github.com/onflow/flow-go/cmd/util/cmd/epochs/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

// generateRecoverEpochTxArgsCmd represents a command to generate the data needed to submit an epoch-recovery transaction
// to the network when it is in EFM (epoch fallback mode).
// EFM can be exited only by a special service event, EpochRecover, which initially originates from a manual service account transaction.
// The full epoch data must be generated manually and submitted with this transaction in order for an
// EpochRecover event to be emitted. This command retrieves the current protocol state identities, computes the cluster assignment using those
// identities, generates the cluster QCs and retrieves the DKG key vector of the last successful epoch.
// This recovery process has some constraints:
//   - The RecoveryEpoch must have exactly the same consensus committee as participated in the most recent successful DKG.
//   - The RecoveryEpoch must contain enough "internal" collection nodes so that all clusters contain a supermajority of "internal" collection nodes (same constraint as sporks)
var (
	generateRecoverEpochTxArgsCmd = &cobra.Command{
		Use:   "efm-recover-tx-args",
		Short: "Generates recover epoch transaction arguments",
		Long: `
Generates transaction arguments for the epoch recovery transaction.
The epoch recovery transaction is used to recover from any failure in the epoch transition process without requiring a spork.
This recovery process has some constraints:
- The RecoveryEpoch must have exactly the same consensus committee as participated in the most recent successful DKG.
- The RecoveryEpoch must contain enough "internal" collection nodes so that all clusters contain a supermajority of "internal" collection nodes (same constraint as sporks)
`,
		Run: generateRecoverEpochTxArgs(getSnapshot),
	}

	flagAnAddress                string
	flagAnPubkey                 string
	flagInternalNodePrivInfoDir  string
	flagNodeConfigJson           string
	flagCollectionClusters       int
	flagNumViewsInEpoch          uint64
	flagNumViewsInStakingAuction uint64
	flagEpochCounter             uint64
)

func init() {
	rootCmd.AddCommand(generateRecoverEpochTxArgsCmd)
	err := addGenerateRecoverEpochTxArgsCmdFlags()
	if err != nil {
		panic(err)
	}
}

func addGenerateRecoverEpochTxArgsCmdFlags() error {
	generateRecoverEpochTxArgsCmd.Flags().IntVar(&flagCollectionClusters, "collection-clusters", 0,
		"number of collection clusters")
	// required parameters for network configuration and generation of root node identities
	generateRecoverEpochTxArgsCmd.Flags().StringVar(&flagNodeConfigJson, "config", "",
		"path to a JSON file containing multiple node configurations (fields Role, Address, Weight)")
	generateRecoverEpochTxArgsCmd.Flags().StringVar(&flagInternalNodePrivInfoDir, "internal-priv-dir", "", "path to directory "+
		"containing the output from the `keygen` command for internal nodes")
	generateRecoverEpochTxArgsCmd.Flags().Uint64Var(&flagNumViewsInEpoch, "epoch-length", 0, "length of each epoch measured in views")
	generateRecoverEpochTxArgsCmd.Flags().Uint64Var(&flagNumViewsInStakingAuction, "epoch-staking-phase-length", 0, "length of the epoch staking phase measured in views")
	generateRecoverEpochTxArgsCmd.Flags().Uint64Var(&flagEpochCounter, "epoch-counter", 0, "the epoch counter used to generate the root cluster block")

	err := generateRecoverEpochTxArgsCmd.MarkFlagRequired("epoch-length")
	if err != nil {
		return fmt.Errorf("failed to mark epoch-length flag as required")
	}
	err = generateRecoverEpochTxArgsCmd.MarkFlagRequired("epoch-staking-phase-length")
	if err != nil {
		return fmt.Errorf("failed to mark epoch-staking-phase-length flag as required")
	}
	err = generateRecoverEpochTxArgsCmd.MarkFlagRequired("epoch-counter")
	if err != nil {
		return fmt.Errorf("failed to mark epoch-counter flag as required")
	}
	err = generateRecoverEpochTxArgsCmd.MarkFlagRequired("collection-clusters")
	if err != nil {
		return fmt.Errorf("failed to mark collection-clusters flag as required")
	}
	return nil
}

func getSnapshot() *inmem.Snapshot {
	// get flow client with secure client connection to download protocol snapshot from access node
	config, err := common.NewFlowClientConfig(flagAnAddress, flagAnPubkey, flow.ZeroID, false)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create flow client config")
	}

	flowClient, err := common.FlowClient(config)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create flow client")
	}

	snapshot, err := common.GetSnapshot(context.Background(), flowClient)
	if err != nil {
		log.Fatal().Err(err).Msg("failed")
	}

	return snapshot
}

// generateRecoverEpochTxArgs generates recover epoch transaction arguments from a root protocol state snapshot and writes it to a JSON file
func generateRecoverEpochTxArgs(getSnapshot func() *inmem.Snapshot) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		stdout := cmd.OutOrStdout()

		// extract arguments from recover epoch tx from snapshot
		txArgs := extractRecoverEpochArgs(getSnapshot())

		// encode to JSON
		encodedTxArgs, err := epochcmdutil.EncodeArgs(txArgs)
		if err != nil {
			log.Fatal().Err(err).Msg("could not encode recover epoch transaction arguments")
		}

		// write JSON args to stdout
		_, err = stdout.Write(encodedTxArgs)
		if err != nil {
			log.Fatal().Err(err).Msg("could not write jsoncdc encoded arguments")
		}
	}
}

// extractRecoverEpochArgs extracts the required transaction arguments for the `recoverEpoch` transaction.
func extractRecoverEpochArgs(snapshot *inmem.Snapshot) []cadence.Value {
	epoch := snapshot.Epochs().Current()

	currentEpochIdentities, err := snapshot.Identities(filter.IsValidProtocolParticipant)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get  valid protocol participants from snapshot")
	}

	// separate collector nodes by internal and partner nodes
	collectors := currentEpochIdentities.Filter(filter.HasRole[flow.Identity](flow.RoleCollection))
	internalCollectors := make(flow.IdentityList, 0)
	partnerCollectors := make(flow.IdentityList, 0)

	log.Info().Msg("collecting internal node network and staking keys")
	internalNodes, err := common.ReadFullInternalNodeInfos(log, flagInternalNodePrivInfoDir, flagNodeConfigJson)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read full internal node infos")
	}

	internalNodesMap := make(map[flow.Identifier]struct{})
	for _, node := range internalNodes {
		if !currentEpochIdentities.Exists(node.Identity()) {
			log.Fatal().Msg(fmt.Sprintf("node ID found in internal node infos missing from protocol snapshot identities: %s", node.NodeID))
		}
		internalNodesMap[node.NodeID] = struct{}{}
	}
	log.Info().Msg("")

	for _, collector := range collectors {
		if _, ok := internalNodesMap[collector.NodeID]; ok {
			internalCollectors = append(internalCollectors, collector)
		} else {
			partnerCollectors = append(partnerCollectors, collector)
		}
	}

	currentEpochDKG, err := epoch.DKG()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get DKG for current epoch")
	}

	log.Info().Msg("computing collection node clusters")

	assignments, clusters, err := common.ConstructClusterAssignment(log, partnerCollectors, internalCollectors, flagCollectionClusters)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to generate cluster assignment")
	}
	log.Info().Msg("")

	log.Info().Msg("constructing root blocks for collection node clusters")
	clusterBlocks := run.GenerateRootClusterBlocks(flagEpochCounter, clusters)
	log.Info().Msg("")

	log.Info().Msg("constructing root QCs for collection node clusters")
	clusterQCs := common.ConstructRootQCsForClusters(log, clusters, internalNodes, clusterBlocks)
	log.Info().Msg("")

	dkgPubKeys := make([]cadence.Value, 0)
	nodeIds := make([]cadence.Value, 0)

	// NOTE: The RecoveryEpoch will re-use the last successful DKG output. This means that the consensus
	// committee in the RecoveryEpoch must be identical to the committee which participated in that DKG.
	dkgGroupKeyCdc, cdcErr := cadence.NewString(currentEpochDKG.GroupKey().String())
	if cdcErr != nil {
		log.Fatal().Err(cdcErr).Msg("failed to get dkg group key cadence string")
	}
	dkgPubKeys = append(dkgPubKeys, dkgGroupKeyCdc)
	for _, id := range currentEpochIdentities {
		if id.GetRole() == flow.RoleConsensus {
			dkgPubKey, keyShareErr := currentEpochDKG.KeyShare(id.GetNodeID())
			if keyShareErr != nil {
				log.Fatal().Err(keyShareErr).Msg(fmt.Sprintf("failed to get dkg pub key share for node: %s", id.GetNodeID()))
			}
			dkgPubKeyCdc, cdcErr := cadence.NewString(dkgPubKey.String())
			if cdcErr != nil {
				log.Fatal().Err(cdcErr).Msg(fmt.Sprintf("failed to get dkg pub key cadence string for node: %s", id.GetNodeID()))
			}
			dkgPubKeys = append(dkgPubKeys, dkgPubKeyCdc)
		}
		nodeIdCdc, err := cadence.NewString(id.GetNodeID().String())
		if err != nil {
			log.Fatal().Err(err).Msg(fmt.Sprintf("failed to convert node ID to cadence string: %s", id.GetNodeID()))
		}
		nodeIds = append(nodeIds, nodeIdCdc)
	}

	// @TODO: cluster qcs are converted into flow.ClusterQCVoteData types,
	// we need a corresponding type in cadence on the FlowClusterQC contract
	// to store this struct.
	_, err = common.ConvertClusterQcsCdc(clusterQCs, clusters)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to convert cluster qcs to cadence type")
	}

	currEpochFinalView, err := epoch.FinalView()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get final view of current epoch")
	}

	args := []cadence.Value{
		// epoch start view
		cadence.NewUInt64(currEpochFinalView + 1),
		// staking phase end view
		cadence.NewUInt64(currEpochFinalView + flagNumViewsInStakingAuction),
		// epoch end view
		cadence.NewUInt64(currEpochFinalView + flagNumViewsInEpoch),
		// dkg pub keys
		cadence.NewArray(dkgPubKeys),
		// node ids
		cadence.NewArray(nodeIds),
		// clusters,
		common.ConvertClusterAssignmentsCdc(assignments),
	}

	return args
}
