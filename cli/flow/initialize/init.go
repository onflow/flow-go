package initialize

import (
	"fmt"
	"github.com/dapperlabs/flow-go/cli"
	"log"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

type Config struct {
	RootKey string `flag:"root-key" info:"root account key"`
	Reset   bool   `default:"false" flag:"reset" info:"reset flow.json config file"`
}

var (
	conf Config
)

var Cmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new account profile",
	Run: func(cmd *cobra.Command, args []string) {
		if !cli.ConfigExists() || conf.Reset {
			var pconf *cli.Config
			if len(conf.RootKey) > 0 {
				prKey := cli.MustDecodeAccountPrivateKeyHex(conf.RootKey)
				pconf = InitProjectWithRootKey(prKey)
			} else {
				pconf = InitProject()
			}
			rootAcct := pconf.RootAccount()

			fmt.Printf("⚙️   Flow client initialized with root account:\n\n")
			fmt.Printf("👤  Address: 0x%x\n", rootAcct.Address.Bytes())
			fmt.Printf("ℹ️   Start the emulator with this root account by running: flow emulator start\n")
		} else {
			fmt.Printf("⚠️   Flow configuration file already exists! Begin by running: flow emulator start\n")
		}
	},
}

// InitProject generates a new root key and saves project config.
func InitProject() *cli.Config {
	prKey, err := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256, []byte{})
	if err != nil {
		cli.Exitf(1, "Failed to generate private key err: %v", err)
	}

	return InitProjectWithRootKey(prKey)
}

// InitProjectWithRootKey creates and saves a new project config
// using the specified root key.
func InitProjectWithRootKey(rootKey flow.AccountPrivateKey) *cli.Config {
	pconf := cli.NewConfig()
	pconf.SetRootAccount(rootKey)
	cli.MustSaveConfig(pconf)
	return pconf
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
