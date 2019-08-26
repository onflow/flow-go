package generate

import (
	"encoding/hex"
	"fmt"
	"log"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type Config struct {
	Seed string `default:"elephant ears" flag:"seed,s"`
}

var conf Config

var Cmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a new key-pair",
	Run: func(cmd *cobra.Command, args []string) {
		salg, _ := crypto.NewSignatureAlgo(crypto.ECDSA_P256)
		prKey, err := salg.GeneratePrKey([]byte(conf.Seed))
		if err != nil {
			fmt.Printf("Failed to generate private key\n")
			return
		}

		prKeyBytes, err := salg.EncodePrKey(prKey)
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
		BindFlags(Cmd.PersistentFlags()).
		Parse()
	if err != nil {
		log.Fatal(err)
	}
}
