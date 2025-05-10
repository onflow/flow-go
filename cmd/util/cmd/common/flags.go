package common

import (
	"github.com/rs/zerolog/log"
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
// this function is usually used in the init() function of a cmd
func InitWithDBFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&flagBadgerDir, "datadir", "d", DefaultBadgerDir,
		"directory to the badger dababase")

	cmd.PersistentFlags().StringVar(&flagPebbleDir, "pebble-dir", DefaultPebbleDir,
		"directory to the pebble dababase")

	cmd.PersistentFlags().StringVar(&flagUseDB, "use-db", DefaultDB, "the database type to use, --badger or --pebble")
}

// ReadDBFlags is to read the database flags
// Note `InitWithDBFlags` only defines the flags, it won't update the flags yet.
// The flags is not updated until cobra.Command.Run is called, so use this function
// only in the cobra.Command.Run function
func ReadDBFlags() DBFlags {
	log.Info().
		Str("datadir", flagBadgerDir).
		Str("pebble-dir", flagPebbleDir).
		Str("use-db", flagUseDB).
		Msgf("read initialized db flags")

	return DBFlags{
		BadgerDir: flagBadgerDir,
		PebbleDir: flagPebbleDir,
		UseDB:     flagUseDB,
	}
}
