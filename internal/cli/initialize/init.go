package initialize

import (
	"fmt"
	"log"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
	"github.com/dapperlabs/bamboo-node/sdk/accounts"
)

type Config struct {
	Seed    string `default:"elephant ears" flag:"seed,s"`
	OutFile string `default:"bamboo.json" flag:"out,o" info:"path to write acount to"`
}

var conf Config

var Cmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new account profile",
	Run: func(cmd *cobra.Command, args []string) {
		salg, _ := crypto.NewSignatureAlgo(crypto.ECDSA_P256)
		prKey, _ := salg.GeneratePrKey([]byte(conf.Seed))

		accountKey := &types.AccountKey{
			Account: types.HexToAddress("01"),
			Key:     prKey,
		}

		err := accounts.SaveAccountToFile(accountKey, conf.OutFile)
		if err != nil {
			fmt.Printf("Failed to write profile to file: %s\n", err.Error())
		}

		fmt.Printf("New profile saved to %s\n", conf.OutFile)
	},
}

func init() {
	initConfig()
}

func initConfig() {
	err := sconfig.New(&conf).
		BindFlags(Cmd.PersistentFlags()).
		Parse()
	if err != nil {
		log.Fatal(err)
	}
}
