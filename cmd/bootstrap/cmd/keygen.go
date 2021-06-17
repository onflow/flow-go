package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/model/bootstrap"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
)

var flagDefaultMachineAccount bool

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

		// if specified, write machine account key files
		// this should be only be used on non-production networks and immediately after
		// bootstrapping from a fresh execution state (eg. benchnet)
		if flagDefaultMachineAccount {
			log.Info().Msg("writing default machine account files")
			genDefaultMachineAccountKeys(nodes)
			log.Info().Msg("")
		}

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
	keygenCmd.Flags().StringVar(&flagConfig, "config", "node-config.json", "path to a JSON file containing multiple node configurations (Role, Address, Stake)")
	_ = keygenCmd.MarkFlagRequired("config")
	keygenCmd.Flags().BoolVar(&flagDefaultMachineAccount, "machine-account", false, "whether or not to generate a default (same as networking key) machine account key file")
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

// Generates machine account key files using the node's networking key and assuming
// a fresh empty execution state
// TODO copied from testnet/network.go
func genDefaultMachineAccountKeys(nodes []model.NodeInfo) {
	nodes = model.Sort(nodes, order.Canonical)

	addressIndex := uint64(4)
	for _, nodeInfo := range nodes {

		if nodeInfo.Role == flow.RoleCollection || nodeInfo.Role == flow.RoleConsensus {
			addressIndex += 2
		} else {
			addressIndex += 1
			continue
		}

		private, err := nodeInfo.Private()
		if err != nil {
			log.Fatal().Err(err).Msg("could not get node info private keys")
		}

		accountAddress, err := flow.Testnet.Chain().AddressAtIndex(addressIndex)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to get address")
		}

		info := model.NodeMachineAccountInfo{
			Address:           accountAddress.HexWithPrefix(),
			EncodedPrivateKey: private.NetworkPrivKey.Encode(),
			KeyIndex:          0,
			SigningAlgorithm:  private.NetworkPrivKey.Algorithm(),
			HashAlgorithm:     crypto.SHA3_256,
		}

		writeJSON(fmt.Sprintf(bootstrap.PathNodeMachineAccountInfoPriv, nodeInfo.NodeID), info)
		if err != nil {
			log.Fatal().Err(err).Msg("could not write machine account")
		}
	}
}
