package initialize

import (
	"encoding/hex"
	"fmt"
	"log"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-node/internal/cli/project"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

type Config struct {
	Reset bool `default:"false" flag:"reset" info:"reset bamboo.json config file"`
}

var (
	conf Config
)

var Cmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new account profile",
	Run: func(cmd *cobra.Command, args []string) {
		if !project.ConfigExists() || conf.Reset {
			salg, _ := crypto.NewSignatureAlgo(crypto.ECDSA_P256)
			prKey, _ := salg.GeneratePrKey([]byte{})
			prKeyBytes, _ := salg.EncodePrKey(prKey)
			prKeyHex := hex.EncodeToString(prKeyBytes)
			address := types.HexToAddress("01").Hex()

			conf := &project.Config{
				Accounts: map[string]*project.AccountConfig{
					"root": &project.AccountConfig{
						Address:    address,
						PrivateKey: prKeyHex,
					},
				},
			}

			project.SaveConfig(conf)

			fmt.Printf("⚙️   Bamboo Client initialized with root account:\n\n")
			fmt.Printf("👤  Address: 0x%s\n", address)
			fmt.Printf("🔑  PrivateKey: %s\n\n", prKeyHex)
			fmt.Printf("ℹ️   Start the emulator with this root account by running: bamboo emulator start\n")
		} else {
			fmt.Printf("⚠️   Bamboo configuration file already exists! Begin by running: bamboo emulator start\n")
		}
	},
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
