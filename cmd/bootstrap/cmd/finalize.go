package cmd

import (
	"github.com/spf13/cobra"
)

// finalizeCmd represents the finalize command
var finalizeCmd = &cobra.Command{
	Use:   "finalize",
	Short: "Finalizes the bootstrapping process",
	Long: `Finalizes the bootstrapping process, which includes generating of internal networking and staking keys,
		running the DKG for generating the random beacon keys, generating genesis execution state, seal, block and
		QC.`,
	Run: func(cmd *cobra.Command, args []string) {
		internalNodesPub, internalNodesPriv := genNetworkAndStakingKeys()

		// TODO assemblePartnerNodes
		// TODO collectStakingInformation
		stakingNodes := internalNodesPub // TODO replace

		dkgDataPub, dkgDataPriv := runDKG(filterConsensusNodes(stakingNodes))

		stateCommitment := genGenesisExecutionState()

		block := constructGenesisBlock(stateCommitment, stakingNodes, dkgDataPub)

		constructGenesisQC(&block, filterConsensusNodes(stakingNodes),
			filterConsensusNodesPriv(internalNodesPriv),
			dkgDataPriv)
	},
}

func init() {
	rootCmd.AddCommand(finalizeCmd)

	finalizeCmd.Flags().StringVarP(&configFile, "config", "c", "",
		"Path to a JSON file containing multiple node configurations (node_role, network_address, stake) [required]")
	finalizeCmd.MarkFlagRequired("config")
}
