package cmd

import (
	"encoding/hex"
	"fmt"
	"path"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	model "github.com/onflow/flow-go/model/bootstrap"
)

// observerNetworkKeyCmd represents the `observer-network-key` command which generates required network key
// for an observer,  and writes it to the default path within the provided directory. Used by new observer
// operators to create the networking key only
var observerNetworkKeyCmd = &cobra.Command{
	Use:   "observer-network-key",
	Short: "Generates network key and writes it to the default path within the output directory",
	Run:   observerNetworkKeyRun,
}

func init() {
	rootCmd.AddCommand(observerNetworkKeyCmd)

	observerNetworkKeyCmd.Flags().BytesHexVar(&flagNetworkSeed, "seed", []byte{}, fmt.Sprintf("hex encoded network key seed (min %d bytes)", minSeedBytes))
}

// observerNetworkKeyRun generate a network key and writes it to a default file path.
func observerNetworkKeyRun(_ *cobra.Command, _ []string) {

	// generate seed if not specified via flag
	if len(flagNetworkSeed) == 0 {
		flagNetworkSeed = GenerateRandomSeed()
	}

	// if the file already exists, exit
	keyExists, err := pathExists(path.Join(flagOutdir, model.FilenameObserverNetworkKey))
	if err != nil {
		log.Fatal().Err(err).Msg("could not check if network-key exists")
	}

	if keyExists {
		log.Warn().Msg("network key already exists, exiting...")
		return
	}

	// generate unstaked networking private key
	seed := validateSeed(flagNetworkSeed)
	networkKey, err := utils.GenerateUnstakedNetworkingKey(seed)
	if err != nil {
		log.Fatal().Err(err).Msg("could not generate network key")
	}
	log.Info().Msg("generated network key")

	// hex encode and write to file
	output := make([]byte, hex.EncodedLen(networkKey.Size()))
	hex.Encode(output, networkKey.Encode())

	writeText(model.FilenameObserverNetworkKey, output)
}
