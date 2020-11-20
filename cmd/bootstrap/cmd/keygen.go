package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	model "github.com/onflow/flow-go/model/bootstrap"
)

// keygenCmd represents the key gen command
var keygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate Staking and Networking keys for a list of nodes",
	Long:  `Generate Staking and Networking keys for a list of nodes provided by the flag '--config'`,
	Run: func(cmd *cobra.Command, args []string) {
		// check if out directory exists
		exists, err := pathExists(flagOutdir)
		if err != nil {
			log.Error().Msg("could not check if directory exists")
			return
		}

		// check if the out directory is empty or has contents
		if exists {
			empty, err := isEmptyDir(flagOutdir)
			if err != nil {
				log.Error().Msg("could not check if directory as empty")
				return
			}

			if !empty {
				log.Error().Msg("output directory already exists and has content. delete and try again.")
				return
			}
		}

		// create keys
		log.Info().Msg("generating internal private networking and staking keys")
		nodes := genNetworkAndStakingKeys()
		log.Info().Msg("")

		// count roles
		roleCounts := nodeCountByRole(nodes)
		for role, count := range roleCounts {
			log.Info().Msg(fmt.Sprintf("created keys for %d %s nodes", count, role.String()))
		}

		log.Info().Msg("generating node public information")
		genNodePubInfo(nodes)
	},
}

func init() {
	rootCmd.AddCommand(keygenCmd)

	// required parameters
	keygenCmd.Flags().
		StringVar(&flagConfig, "config", "--node-config.json", "path to a JSON file containing multiple node configurations (Role, Address, Stake)")
	_ = keygenCmd.MarkFlagRequired("config")

}

// isEmptyDir returns True if the directory contains children
func isEmptyDir(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err // Either not empty or error, suits both cases
}

func genNodePubInfo(nodes []model.NodeInfo) {
	pubNodes := make([]model.NodeInfoPub, 0, len(nodes))
	for _, node := range nodes {
		pubNodes = append(pubNodes, node.Public())
	}
	writeJSON(model.PathInternalNodeInfosPub, pubNodes)
}
