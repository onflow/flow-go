package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	epochcmdutil "github.com/onflow/flow-go/cmd/util/cmd/epochs/utils"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/grpcclient"
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

	flagOut                      string
	flagAnAddress                string
	flagAnPubkey                 string
	flagAnInsecure               bool
	flagInternalNodePrivInfoDir  string
	flagNodeConfigJson           string
	flagCollectionClusters       int
	flagNumViewsInEpoch          uint64
	flagNumViewsInStakingAuction uint64
	flagEpochCounter             uint64
	flagTargetDuration           uint64
	flagTargetEndTime            uint64
	flagInitNewEpoch             bool
	flagRootChainID              string
)

func init() {
	rootCmd.AddCommand(generateRecoverEpochTxArgsCmd)
	err := addGenerateRecoverEpochTxArgsCmdFlags()
	if err != nil {
		panic(err)
	}
}

func addGenerateRecoverEpochTxArgsCmdFlags() error {
	generateRecoverEpochTxArgsCmd.Flags().StringVar(&flagOut, "out", "", "file to write tx args output")
	generateRecoverEpochTxArgsCmd.Flags().StringVar(&flagAnAddress, "access-address", "", "the address of the access node used for client connections")
	generateRecoverEpochTxArgsCmd.Flags().StringVar(&flagRootChainID, "root-chain-id", "", "the root chain id")
	generateRecoverEpochTxArgsCmd.Flags().StringVar(&flagAnPubkey, "access-network-key", "", "the network key of the access node used for client connections in hex string format")
	generateRecoverEpochTxArgsCmd.Flags().BoolVar(&flagAnInsecure, "insecure", false, "set to true if the protocol snapshot should be retrieved from the insecure AN endpoint")
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
	generateRecoverEpochTxArgsCmd.Flags().Uint64Var(&flagTargetDuration, "epoch-timing-duration", 0, "the target duration of the epoch, in seconds")
	generateRecoverEpochTxArgsCmd.Flags().Uint64Var(&flagTargetEndTime, "epoch-timing-end-time", 0, "the target end time for the epoch, specified in second-precision Unix time")
	// The following option allows the RecoveryEpoch specified by this command to overwrite an epoch which already exists in the smart contract.
	// This is needed only if a previous recoverEpoch transaction was submitted and a race condition occurred such that:
	//   - the RecoveryEpoch in the admin transaction was accepted by the smart contract
	//   - the RecoveryEpoch service event (after sealing latency) was rejected by the Protocol State
	generateRecoverEpochTxArgsCmd.Flags().BoolVar(&flagInitNewEpoch, "unsafe-overwrite-epoch-data", false, "set to true if the resulting transaction is allowed to overwrite an existing epoch data entry in the smart contract. ")

	err := generateRecoverEpochTxArgsCmd.MarkFlagRequired("access-address")
	if err != nil {
		return fmt.Errorf("failed to mark access-address flag as required")
	}
	err = generateRecoverEpochTxArgsCmd.MarkFlagRequired("epoch-length")
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
	err = generateRecoverEpochTxArgsCmd.MarkFlagRequired("epoch-timing-duration")
	if err != nil {
		return fmt.Errorf("failed to mark target-duration flag as required")
	}
	err = generateRecoverEpochTxArgsCmd.MarkFlagRequired("epoch-timing-end-time")
	if err != nil {
		return fmt.Errorf("failed to mark target-end-time flag as required")
	}
	err = generateRecoverEpochTxArgsCmd.MarkFlagRequired("root-chain-id")
	if err != nil {
		return fmt.Errorf("failed to mark root-chain-id flag as required")
	}
	return nil
}

func getSnapshot() *inmem.Snapshot {
	// get flow client with secure client connection to download protocol snapshot from access node
	config, err := grpcclient.NewFlowClientConfig(flagAnAddress, flagAnPubkey, flow.ZeroID, flagAnInsecure)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create flow client config")
	}

	flowClient, err := grpcclient.FlowClient(config)
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
		// generate transaction arguments
		txArgs, err := run.GenerateRecoverEpochTxArgs(
			log,
			flagInternalNodePrivInfoDir,
			flagNodeConfigJson,
			flagCollectionClusters,
			flagEpochCounter,
			flow.ChainID(flagRootChainID),
			flagNumViewsInStakingAuction,
			flagNumViewsInEpoch,
			flagTargetDuration,
			flagTargetEndTime,
			flagInitNewEpoch,
			getSnapshot(),
		)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to generate recover epoch transaction arguments")
		}
		// encode to JSON
		encodedTxArgs, err := epochcmdutil.EncodeArgs(txArgs)
		if err != nil {
			log.Fatal().Err(err).Msg("could not encode recover epoch transaction arguments")
		}

		if flagOut == "" {
			// write JSON args to stdout
			_, err = cmd.OutOrStdout().Write(encodedTxArgs)
			if err != nil {
				log.Fatal().Err(err).Msg("could not write jsoncdc encoded arguments")
			}
		} else {
			// write JSON args to file specified by flag
			err := os.WriteFile(flagOut, encodedTxArgs, 0644)
			if err != nil {
				log.Fatal().Err(err).Msg(fmt.Sprintf("could not write jsoncdc encoded arguments to file %s", flagOut))
			}
			log.Info().Msgf("wrote transaction args to output file %s", flagOut)
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
	clusterQCs := run.ConstructRootQCsForClusters(log, clusters, internalNodes, clusterBlocks)
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

	clusterQCAddress := systemcontracts.SystemContractsForChain(flow.ChainID(flagRootChainID)).ClusterQC.Address.String()
	qcVoteData, err := common.ConvertClusterQcsCdc(clusterQCs, clusters, clusterQCAddress)
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
		// target duration
		cadence.NewUInt64(flagTargetDuration),
		// target end time
		cadence.NewUInt64(flagTargetEndTime),
		// clusters,
		common.ConvertClusterAssignmentsCdc(assignments),
		// qcVoteData
		cadence.NewArray(qcVoteData),
		// dkg pub keys
		cadence.NewArray(dkgPubKeys),
		// node ids
		cadence.NewArray(nodeIds),
		// recover the network by initializing a new recover epoch which will increment the smart contract epoch counter
		// or overwrite the epoch metadata for the current epoch
		cadence.NewBool(flagInitNewEpoch),
	}

	return args
}
