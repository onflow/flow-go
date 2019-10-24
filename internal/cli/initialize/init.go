package initialize

import (
	"encoding/hex"
	"fmt"
	"log"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/internal/cli/project"
	"github.com/dapperlabs/flow-go/internal/cli/utils"
	"github.com/dapperlabs/flow-go/pkg/types"
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
			rootAcct := pconf.Accounts["root"]

			fmt.Printf("⚙️   Flow client initialized with root account:\n\n")
			fmt.Printf("👤  Address: 0x%s\n", rootAcct.Address)
			fmt.Printf("🔑  PrivateKey: %s\n\n", rootAcct.PrivateKeyHex)
			fmt.Printf("ℹ️   Start the emulator with this root account by running: flow emulator start\n")
		} else {
			fmt.Printf("⚠️   Flow configuration file already exists! Begin by running: flow emulator start\n")
		}
	},
}

// InitProject generates a new root key and saves project config.
func InitProject() *project.Config {
	privateKey, err := keys.GeneratePrivateKey(keys.KeyTypeECDSA_P256_SHA3_256, []byte{})
	if err != nil {
		utils.Exitf(1, "Failed to generate private key: %v", err)
	}

	privateKeyBytes, err := types.EncodeAccountPrivateKey(privateKey)
	if err != nil {
		utils.Exitf(1, "Failed to encode private key: %v", err)
	}

	privateKeyHex := hex.EncodeToString(privateKeyBytes)
	address := types.HexToAddress("01").Hex()

	conf := &project.Config{
		Accounts: map[string]*project.AccountConfig{
			"root": {
				Address:       address,
				PrivateKeyHex: privateKeyHex,
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
