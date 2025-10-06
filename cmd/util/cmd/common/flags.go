package common

import (
	"github.com/spf13/cobra"
)

const DefaultProtocolDBDir = "/var/flow/data/protocol"

// InitDataDirFlag initializes the --datadir flag on the given command.
func InitDataDirFlag(cmd *cobra.Command, dataDirFlag *string) {
	cmd.PersistentFlags().StringVarP(dataDirFlag, "datadir", "d", DefaultProtocolDBDir,
		"directory to the protocol db dababase")
}
