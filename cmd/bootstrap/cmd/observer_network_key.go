package cmd

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/onflow/crypto"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
)

var (
	flagOutputFile string
)

// observerNetworkKeyCmd represents the `observer-network-key` command which generates required
// network key for an Observer, and writes it to the provided path. Used by new Observer operators
// to create the networking key only
var observerNetworkKeyCmd = &cobra.Command{
	Use:   "observer-network-key",
	Short: "Generates network key and writes it to the provided path",
	Run:   observerNetworkKeyRun,
}

func init() {
	rootCmd.AddCommand(observerNetworkKeyCmd)

	observerNetworkKeyCmd.Flags().StringVarP(&flagOutputFile, "output-file", "f", "", "output file path")
	cmd.MarkFlagRequired(observerNetworkKeyCmd, "output-file")

	observerNetworkKeyCmd.Flags().BytesHexVar(
		&flagNetworkSeed,
		"seed",
		[]byte{},
		fmt.Sprintf("hex encoded networking seed (min %d bytes)", crypto.KeyGenSeedMinLen))
}

// observerNetworkKeyRun generate a network key and writes it to the provided file path.
func observerNetworkKeyRun(_ *cobra.Command, _ []string) {

	// generate seed if not specified via flag
	if len(flagNetworkSeed) == 0 {
		flagNetworkSeed = GenerateRandomSeed(crypto.KeyGenSeedMinLen)
	}

	// if the file already exists, exit
	keyExists, err := common.PathExists(flagOutputFile)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not check if %s exists", flagOutputFile)
	}

	if keyExists {
		log.Warn().Msgf("%s already exists, exiting...", flagOutputFile)
		return
	}

	// generate observer networking private key
	networkKey, err := utils.GeneratePublicNetworkingKey(flagNetworkSeed)
	if err != nil {
		log.Fatal().Err(err).Msg("could not generate network key")
	}
	log.Info().Msg("generated network key")

	// hex encode and write to file
	keyBytes := networkKey.Encode()
	output := make([]byte, hex.EncodedLen(len(keyBytes)))
	hex.Encode(output, keyBytes)

	// write to file
	err = os.WriteFile(flagOutputFile, output, 0600)
	if err != nil {
		log.Fatal().Err(err).Msg("could not write file")
	}

	log.Info().Msgf("wrote file %v", flagOutputFile)
}
