package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	model "github.com/onflow/flow-go/model/bootstrap"
)

var flagDefaultMachineAccount bool

// keygenCmd represents the key gen command
var keygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate Staking and Networking keys for a list of nodes",
	Long:  `Generate Staking and Networking keys for a list of nodes provided by the flag '--config'`,
	Run: func(cmd *cobra.Command, args []string) {
		// check if out directory exists
		exists, err := common.PathExists(flagOutdir)
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

		// write key files
		writeJSONFile := func(relativePath string, val interface{}) error {
			return common.WriteJSON(relativePath, flagOutdir, val)
		}
		writeFile := func(relativePath string, data []byte) error {
			return common.WriteText(relativePath, flagOutdir, data)
		}

		log.Info().Msg("writing internal private key files")
		err = utils.WriteStakingNetworkingKeyFiles(nodes, writeJSONFile)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to write internal private key files")
		}
		log.Info().Msg("")

		log.Info().Msg("writing internal db encryption key files")
		err = utils.WriteSecretsDBEncryptionKeyFiles(nodes, writeFile)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to write internal db encryption key files")
		}
		log.Info().Msg("")

		// if specified, write machine account key files
		// this should be only be used on non-production networks and immediately after
		// bootstrapping from a fresh execution state (eg. benchnet)
		if flagDefaultMachineAccount {
			chainID := parseChainID(flagRootChain)
			log.Info().Msg("writing default machine account files")
			err = utils.WriteMachineAccountFiles(chainID, nodes, writeJSONFile)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to write machine account key files")
			}
			log.Info().Msg("")
		}

		// count roles
		roleCounts := common.NodeCountByRole(nodes)
		for role, count := range roleCounts {
			log.Info().Msg(fmt.Sprintf("created keys for %d %s nodes", count, role.String()))
		}

		log.Info().Msg("generating node public information")
		err = writeNodePubInfo(nodes)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to generate nodes public info")
		}
	},
}

func init() {
	rootCmd.AddCommand(keygenCmd)

	// required parameters
	keygenCmd.Flags().StringVar(&flagConfig, "config", "node-config.json", "path to a JSON file containing multiple node configurations (Role, Address, Weight)")
	cmd.MarkFlagRequired(keygenCmd, "config")

	// optional parameters, used for generating machine account files
	keygenCmd.Flags().BoolVar(&flagDefaultMachineAccount, "machine-account", false, "whether or not to generate a default (same as networking key) machine account key file")
	keygenCmd.Flags().StringVar(&flagRootChain, "root-chain", "local", "chain ID for the root block (can be 'main', 'test', 'sandbox', 'preview', 'bench', or 'local'")
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

func writeNodePubInfo(nodes []model.NodeInfo) error {
	pubNodes := make([]model.NodeInfoPub, 0, len(nodes))
	for _, node := range nodes {
		pub, err := node.Public()
		if err != nil {
			return fmt.Errorf("failed to read public info: %w", err)
		}
		pubNodes = append(pubNodes, pub)
	}
	err := common.WriteJSON(model.PathInternalNodeInfosPub, flagOutdir, pubNodes)
	if err != nil {
		return err
	}
	log.Info().Msgf("wrote file %s/%s", flagOutdir, model.PathInternalNodeInfosPub)
	return nil
}
