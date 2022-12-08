package cmd

import (
	"encoding/json"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/signature"
)

var (
	flagSignerIndices string
)

var DecodeSignersCmd = &cobra.Command{
	Use:   "decode-signer-indices",
	Short: "Decode consensus node signer indices",
	Run:   runDecodeSigners,
}

func init() {
	rootCmd.AddCommand(DecodeSignersCmd)

	DecodeSignersCmd.Flags().StringVar(&flagSignerIndices, "indices", "", "Signer indices, base64-encoded")
}

func runDecodeSigners(*cobra.Command, []string) {
	db := common.InitStorage(flagDatadir)
	defer db.Close()

	storages := common.InitStorages(db)
	state, err := common.InitProtocolState(db, storages)
	if err != nil {
		log.Fatal().Err(err).Msg("could not init protocol state")
	}

	var signerIndices []byte
	err = json.Unmarshal([]byte("\""+flagSignerIndices+"\""), &signerIndices)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not decode signer indices from base64: %s", flagSignerIndices)
	}

	// always use current epoch
	epoch := state.Final().Epochs().Current()
	identities, err := epoch.InitialIdentities()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get epoch identities")
	}
	epochCommittee := identities.Filter(filter.IsVotingConsensusCommitteeMember)

	signers, err := signature.DecodeSignerIndicesToIdentities(epochCommittee, signerIndices)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not decode signer indices to identities: %x", signerIndices)
	}

	log.Info().
		Strs("signer_ids", signers.NodeIDs().Strings()).
		Msgf("successfully decoded signer indices, printing ")

}
