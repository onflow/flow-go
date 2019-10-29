package generate

import (
	"encoding/hex"
	"fmt"
	"log"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cli/utils"
	"github.com/dapperlabs/flow-go/crypto"
)

type Config struct {
	Seed string `default:"elephant ears" flag:"seed,s"`
}

var conf Config

var Cmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a new key-pair",
	Run: func(cmd *cobra.Command, args []string) {
		prKey, err := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte(conf.Seed))
		if err != nil {
			fmt.Printf("Failed to generate private key\n")
			return
		}

		prKeyBytes, err := prKey.Encode()
		if err != nil {
			fmt.Printf("Failed to encode private key\n")
			return
		}

		prKeyHex := hex.EncodeToString(prKeyBytes)

		fmt.Printf("Generated a new private key:\n")
		fmt.Printf("%s\n", prKeyHex)
	},
}

func init() {
	initConfig()
}

func initConfig() {
	err := sconfig.New(&conf).
		FromEnvironment(utils.EnvPrefix).
		BindFlags(Cmd.PersistentFlags()).
		Parse()
	if err != nil {
		log.Fatal(err)
	}
}
