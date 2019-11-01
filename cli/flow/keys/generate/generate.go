package generate

import (
	"fmt"
	"log"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cli"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

type Config struct {
	Seed string `flag:"seed,s" info:"deterministic seed phrase"`
}

var conf Config

var Cmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a new key-pair",
	Run: func(cmd *cobra.Command, args []string) {
		var seed []byte
		if conf.Seed == "" {
			seed = cli.RandomSeed(crypto.KeyGenSeedMinLenECDSA_P256)
		} else {
			seed = []byte(conf.Seed)
		}

		privateKey, err := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256, seed)
		if err != nil {
			cli.Exitf(1, "Failed to generate private key: %v", err)
		}

		prKeyBytes, err := keys.EncodePrivateKey(privateKey)
		if err != nil {
			fmt.Printf("Failed to encode private key\n")
			return
		}

		fmt.Printf("Generated a new private key:\n")
		fmt.Printf("%x\n", prKeyBytes)
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
