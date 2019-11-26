package abi

import (
	"log"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cli"
	"github.com/dapperlabs/flow-go/language/runtime/cmd/abi"
)

type Config struct {
	Pretty bool `default:"false" flag:"pretty,p" info:"pretty-prints JSON"`
}

var (
	conf Config
)

var Cmd = &cobra.Command{
	Use:   "abi",
	Short: "Generates JSON ABI from given Cadence file",
	Run: func(cmd *cobra.Command, args []string) {
		abi.GenerateABI(args, conf.Pretty)
	},
}

func init() {
	initConfig()
}

func initConfig() {
	err := sconfig.New(&conf).
		FromEnvironment(cli.EnvPrefix).
		BindFlags(Cmd.PersistentFlags()).
		Parse()
	if err != nil {
		log.Fatal(err)
	}
}
