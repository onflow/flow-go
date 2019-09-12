package initialize

import (
	"encoding/hex"

	"github.com/psiemens/sconfig"
	log "github.com/sirupsen/logrus"
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
			log.WithFields(log.Fields{
				"address": address,
				"prKey":   prKeyHex,
			}).Infof("⚙️   Bamboo Client initialized with root account 0x%s", address)
			log.Info("⚙️   Bamboo Client setup finished! Begin by running: bamboo emulator start")
		} else {
			log.Warn("⚙️   Bamboo configuration file already exists! Begin by running: bamboo emulator start")
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
