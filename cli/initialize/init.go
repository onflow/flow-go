package initialize

import (
	"encoding/hex"
	"fmt"
	"log"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cli/project"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
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
			pconf := InitializeProject()
			rootAcct := pconf.Accounts["root"]

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
		panic(err)
	}
	prKeyBytes, err := prKey.Encode()
	if err != nil {
		panic(err)
	}
	prKeyHex := hex.EncodeToString(prKeyBytes)
	address := flow.HexToAddress("01").Hex()

	conf := &project.Config{
		Accounts: map[string]*project.AccountConfig{
			"root": &project.AccountConfig{
				Address:    address,
				PrivateKey: prKeyHex,
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
