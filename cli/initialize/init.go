package initialize

import (
	"fmt"
	"log"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cli/project"
	"github.com/dapperlabs/flow-go/cli/utils"
	"github.com/dapperlabs/flow-go/model/types"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

type Config struct {
	Reset bool `default:"false" flag:"reset" info:"reset flow.json config file"`
}

var (
	conf Config
)

var Cmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new account profile",
	Run: func(cmd *cobra.Command, args []string) {
		if !project.ConfigExists() || conf.Reset {
			pconf := InitProject()
			rootAcct := pconf.RootAccount()

			fmt.Printf("⚙️   Flow client initialized with root account:\n\n")
			fmt.Printf("👤  Address: 0x%x\n", rootAcct.Address)
			fmt.Printf("ℹ️   Start the emulator with this root account by running: flow emulator start\n")
		} else {
			fmt.Printf("⚠️   Flow configuration file already exists! Begin by running: flow emulator start\n")
		}
	},
}

// InitProject generates a new root key and saves project config.
func InitProject() *project.Config {
	privateKey, err := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256, []byte{})
	if err != nil {
		utils.Exitf(1, "Failed to generate private key: %v", err)
	}

	address := types.HexToAddress("01")

	conf := &project.Config{
		Accounts: map[string]*project.Account{
			"root": {
				Address:    address,
				PrivateKey: privateKey,
			},
		},
	}

	project.SaveConfig(conf)

	return conf
}

func init() {
	initConfig()
}

func initConfig() {
	err := sconfig.New(&conf).
		FromEnvironment("BAM").
		BindFlags(Cmd.PersistentFlags()).
		Parse()
	if err != nil {
		log.Fatal(err)
	}
}
