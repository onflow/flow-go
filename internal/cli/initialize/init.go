package initialize

import (
	"fmt"
	"log"

	"github.com/dapperlabs/flow-go/internal/cli/utils"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/internal/cli/project"
	"github.com/dapperlabs/flow-go/pkg/crypto"
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
		if !project.ConfigExists() || conf.Reset {
			var pconf *project.Config
			if len(conf.RootKey) > 0 {
				prKey, err := utils.DecodePrivateKey(conf.RootKey)
				if err != nil {
					utils.Exitf(1, "Failed to decode root key err: %v", err)
				}
				pconf = InitializeProjectWithRootKey(prKey)
			} else {
				pconf = InitializeProject()
			}
			rootAcct := pconf.RootAccount()

			fmt.Printf("⚙️   Flow client initialized with root account:\n\n")
			fmt.Printf("👤  Address: 0x%s\n", rootAcct.Address)
			fmt.Printf("🔑  PrivateKey: %s\n\n", rootAcct.PrivateKey)
			fmt.Printf("ℹ️   Start the emulator with this root account by running: flow emulator start\n")
		} else {
			fmt.Printf("⚠️   Flow configuration file already exists! Begin by running: flow emulator start\n")
		}
	},
}

// InitializeProject generates a new root key and saves project config.
func InitializeProject() *project.Config {
	prKey, err := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte{})
	if err != nil {
		utils.Exitf(1, "Failed to generate private key err: %v", err)
	}

	return InitializeProjectWithRootKey(prKey)
}

// InitializeProjectWithRootKey creates and saves a new project config
// using the specified root key.
func InitializeProjectWithRootKey(rootKey crypto.PrivateKey) *project.Config {
	pconf := project.NewConfig()
	pconf.SetRootAccount(rootKey)
	project.MustSaveConfig(pconf)
	return pconf
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
