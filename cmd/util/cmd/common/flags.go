package common

import (
	"github.com/spf13/cobra"
)

const DefaultProtocolDBDir = "/var/flow/data/protocol"

// // InitWithDBFlags initializes the command with the database flags
// // and sets the default values for the flags
// // this function is usually used in the init() function of a cmd
// func InitWithDBFlags(cmd *cobra.Command) {
// 	cmd.PersistentFlags().StringVarP(&flagProtocolDBDir, "datadir", "d", DefaultProtocolDBDir,
// 		"directory to the protocol db dababase")
// }

func InitDataDirFlag(cmd *cobra.Command, dataDirFlag *string) {
	cmd.PersistentFlags().StringVarP(dataDirFlag, "datadir", "d", DefaultProtocolDBDir,
		"directory to the protocol db dababase")
}

// ReadDatadirFlag is to read the database flags
// Note `InitWithDBFlags` only defines the flags, it won't update the flags yet.
// The flags is not updated until cobra.Command.Run is called, so use this function
// only in the cobra.Command.Run function
// func ReadDatadirFlag() string {
// 	log.Info().
// 		Str("datadir", flagProtocolDBDir).
// 		Msgf("read initialized protocol db flag")
//
// 	return flagProtocolDBDir
// }
