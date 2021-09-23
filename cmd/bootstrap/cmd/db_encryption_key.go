package cmd

import (
	"fmt"
	"path"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	model "github.com/onflow/flow-go/model/bootstrap"
)

// dbEncryptionKyCmd adds a command to the bootstrap utility which generates an
// AES-256 key for encrypting the secrets database, and writes it to the default
// path.
var dbEncryptionKyCmd = &cobra.Command{
	Use:   "db-encryption-key",
	Short: "Generates encryption key for secrets database and writes it to the default path within the bootstrap directory",
	Run:   dbEncryptionKeyRun,
}

func init() {
	rootCmd.AddCommand(dbEncryptionKyCmd)
}

func dbEncryptionKeyRun(_ *cobra.Command, _ []string) {

	// read nodeID written to boostrap dir by `bootstrap key`
	nodeID, err := readNodeID()
	if err != nil {
		log.Fatal().Err(err).Msg("could not read node id")
	}

	// check if the key already exists
	dbEncryptionKeyPath := fmt.Sprintf(model.PathSecretsEncryptionKey, nodeID)
	exists, err := pathExists(path.Join(flagOutdir, dbEncryptionKeyPath))
	if err != nil {
		log.Fatal().Err(err).Msg("could not check if db encryption key already exists")
	}
	if exists {
		log.Warn().Msg("DB encryption key already exists, exiting...")
		return
	}

	dbEncryptionKey, err := utils.GenerateSecretsDBEncryptionKey()
	if err != nil {
		log.Fatal().Err(err).Msg("could not generate db encryption key")
	}
	log.Info().Msg("generated db encryption key")

	writeText(dbEncryptionKeyPath, dbEncryptionKey)
}
