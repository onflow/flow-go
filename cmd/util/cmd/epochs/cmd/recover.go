package cmd

import (
	"context"
	"encoding/hex"
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

// generateRecoverEpochTxArgsCmd represents a command to generate the data needed to submit an epoch recovery transaction the network is in EFM (epoch fallback mode).
// EFM can be exited only by a special service event, EpochRecover, which initially originates from a manual service account transaction.
// The full epoch data must be generated manually and submitted with this transaction in order for an
// EpochRecover event to be emitted. This command retrieves the current protocol state identities, computes the cluster assignment using those
// identities, generates the cluster QC's and retrieves the DKG key vector of the last successful epoch.
var (
	generateRecoverEpochTxArgsCmd = &cobra.Command{
		Use:   "recover-epoch-tx-args",
		Short: "Generates recover epoch transaction arguments",
		Long:  "Generates transaction arguments for the epoch recovery transaction.",
		Run:   generateRecoverEpochTxArgs(getSnapshot),
	}

	flagAnAddress               string
	flagAnPubkey                string
	flagInternalNodePrivInfoDir string
	flagNodeConfigJson          string
	flagCollectionClusters      int
	flagStartView               uint64
	flagStakingEndView          uint64
	flagEndView                 uint64
)

func init() {
	rootCmd.AddCommand(generateRecoverEpochTxArgsCmd)
	addGenerateRecoverEpochTxArgsCmdFlags()
}

func addGenerateRecoverEpochTxArgsCmdFlags() {
	generateRecoverEpochTxArgsCmd.Flags().IntVar(&flagCollectionClusters, "collection-clusters", 0,
		"number of collection clusters")
	// required parameters for network configuration and generation of root node identities
	generateRecoverEpochTxArgsCmd.Flags().StringVar(&flagNodeConfigJson, "node-config", "",
		"path to a JSON file containing multiple node configurations (fields Role, Address, Weight)")
	generateRecoverEpochTxArgsCmd.Flags().StringVar(&flagInternalNodePrivInfoDir, "internal-priv-dir", "", "path to directory "+
		"containing the output from the `keygen` command for internal nodes")
	generateRecoverEpochTxArgsCmd.Flags().Uint64Var(&flagStartView, "start-view", 0, "start view of the recovery epoch")
	generateRecoverEpochTxArgsCmd.Flags().Uint64Var(&flagStakingEndView, "staking-end-view", 0, "end view of the staking phase of the recovery epoch")
	generateRecoverEpochTxArgsCmd.Flags().Uint64Var(&flagEndView, "end-view", 0, "end view of the recovery epoch")
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

// extractResetEpochArgs extracts the required transaction arguments for the `resetEpoch` transaction
func extractRecoverEpochArgs(snapshot *inmem.Snapshot) []cadence.Value {
	epoch := snapshot.Epochs().Current()

	ids, err := snapshot.Identities(filter.IsValidProtocolParticipant)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get  valid protocol participants from snapshot")
	}

	// separate collector nodes by internal and partner nodes
	collectors := ids.Filter(filter.HasRole[flow.Identity](flow.RoleCollection))
	internalCollectors := make(flow.IdentityList, 0)
	partnerCollectors := make(flow.IdentityList, 0)

	log.Info().Msg("collecting internal node network and staking keys")
	internalNodes := common.ReadInternalNodeInfos(log, flagInternalNodePrivInfoDir, flagNodeConfigJson)
	internalNodesMap := make(map[flow.Identifier]struct{})
	for _, node := range internalNodes {
		if !ids.Exists(node.Identity()) {
			log.Fatal().Msg(fmt.Sprintf("node ID found in internal node infos missing from protocol snapshot identities: %s", node.NodeID))
		}
		internalNodesMap[node.NodeID] = struct{}{}
	}
	log.Info().Msg("")

	collectors.Map(func(identity flow.Identity) flow.Identity {
		if _, ok := internalNodesMap[identity.NodeID]; ok {
			internalCollectors = append(internalCollectors, &identity)
		} else {
			partnerCollectors = append(partnerCollectors, &identity)
		}
		return identity
	})

	currentEpochDKG, err := epoch.DKG()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get DKG for current epoch")
	}

	log.Info().Msg("computing collection node clusters")
	_, clusters, err := common.ConstructClusterAssignment(log, partnerCollectors, internalCollectors, flagCollectionClusters)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to generate cluster assignment")
	}

	log.Info().Msg("")

	epochCounter, err := epoch.Counter()
	if err != nil {
		log.Fatal().Err(err).Msg("unable to get epoch counter from current epoch")
	}
	log.Info().Msg("constructing root blocks for collection node clusters")
	clusterBlocks := run.GenerateRootClusterBlocks(epochCounter, clusters)
	log.Info().Msg("")

	log.Info().Msg("constructing root QCs for collection node clusters")
	clusterQCs := common.ConstructRootQCsForClusters(log, clusters, internalNodes, clusterBlocks)
	log.Info().Msg("")

	randomSource, err := epoch.RandomSource()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get random source for current epoch")
	}
	randomSourceCdc, err := cadence.NewString(hex.EncodeToString(randomSource))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get random source cadence string")
	}

	dkgPubKeys := make([]cadence.Value, 0)
	nodeIds := make([]cadence.Value, 0)
	ids.Map(func(identity flow.Identity) flow.Identity {
		if identity.GetRole() == flow.RoleConsensus {
			dkgPubKey, keyShareErr := currentEpochDKG.KeyShare(identity.GetNodeID())
			if keyShareErr != nil {
				log.Fatal().Err(keyShareErr).Msg(fmt.Sprintf("failed to get dkg pub key share for node: %s", identity.GetNodeID()))
			}
			dkgPubKeyCdc, cdcErr := cadence.NewString(dkgPubKey.String())
			if cdcErr != nil {
				log.Fatal().Err(cdcErr).Msg(fmt.Sprintf("failed to get dkg pub key cadence string for node: %s", identity.GetNodeID()))
			}
			dkgPubKeys = append(dkgPubKeys, dkgPubKeyCdc)
		}
		nodeIdCdc, err := cadence.NewString(identity.GetNodeID().String())
		if err != nil {
			log.Fatal().Err(err).Msg(fmt.Sprintf("failed to convert node ID to cadence string: %s", identity.GetNodeID()))
		}
		nodeIds = append(nodeIds, nodeIdCdc)
		return identity
	})

	// @TODO: cluster qcs are converted into flow.ClusterQCVoteData types,
	// we need a corresponding type in cadence on the FlowClusterQC contract
	// to store this struct.
	_, err = common.ConvertClusterQcsCdc(clusterQCs, clusters)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to convert cluster qcs to cadence type")
	}

	args := []cadence.Value{
		randomSourceCdc,
		cadence.NewUInt64(flagStartView),
		cadence.NewUInt64(flagStakingEndView),
		cadence.NewUInt64(flagEndView),
		cadence.NewArray(dkgPubKeys),
		cadence.NewArray(nodeIds),
		//common.ConvertClusterAssignmentsCdc(assignments),
	}

	return args
}
