package cmd

import (
	"github.com/onflow/flow-go/cmd"
	"github.com/spf13/cobra"
)

var (
	flagPartnerInfosOutDir string
)

// populatePartnerInfos represents the `populate-partner-infos` command which will read the proposed node
// table from the staking contract and for each identity in the proposed table generate a node-info-pub
// json file. It will also generate the partner-stakes json file.
var populatePartnerInfosCMD = &cobra.Command{
	Use:   "populate-partner-infos",
	Short: "Generates a partner-stakes.json file as well as node-info-pub-*.json for all proposed identities in staking contract",
	Run:   machineAccountRun,
}

func init() {
	rootCmd.AddCommand(populatePartnerInfosCMD)

	populatePartnerInfosCMD.Flags().StringVar(&flagPartnerInfosOutDir, "out", "", "the directory where the generated node-info-pub files will be written")
	cmd.MarkFlagRequired(populatePartnerInfosCMD, "out")
}
