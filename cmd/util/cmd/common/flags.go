package common

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	flagBadgerDir string
	flagPebbleDir string
	flagUseDB     string
)

type DBFlags struct {
	BadgerDir string
	PebbleDir string
	UseDB     string
}

const DefaultBadgerDir = "/var/flow/data/protocol"
const DefaultPebbleDir = "/var/flow/data/protocol-pebble"
const DefaultDB = "badger" // "badger|pebble"

// InitWithDBFlags initializes the command with the database flags
// and sets the default values for the flags
func InitWithDBFlags(cmd *cobra.Command) DBFlags {
	cmd.PersistentFlags().StringVarP(&flagBadgerDir, "datadir", "d", DefaultBadgerDir,
		fmt.Sprintf("directory to the badger dababase, default: %s", DefaultBadgerDir))
	// datadir flag does not have to be required, because...

	// TODO: switch to pebbledir
	cmd.PersistentFlags().StringVar(&flagPebbleDir, "pebble-dir", DefaultPebbleDir,
		fmt.Sprintf("directory to the pebble dababase, default: %s", DefaultPebbleDir))

	cmd.PersistentFlags().StringVar(&flagUseDB, "use-db", DefaultDB, "the database type to use, --badger (default) or --pebble")

	return DBFlags{
		BadgerDir: flagBadgerDir,
		PebbleDir: flagPebbleDir,
		UseDB:     flagUseDB,
	}
}
