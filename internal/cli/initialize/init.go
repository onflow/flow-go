package initialize

import (
	"encoding/hex"

	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-node/internal/cli/project"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

var Cmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new account profile",
	Run: func(cmd *cobra.Command, args []string) {
		salg, _ := crypto.NewSignatureAlgo(crypto.ECDSA_P256)
		prKey, _ := salg.GeneratePrKey([]byte{})
		prKeyBytes, _ := salg.EncodePrKey(prKey)
		prKeyHex := hex.EncodeToString(prKeyBytes)

		conf := &project.Config{
			Accounts: map[string]*project.AccountConfig{
				"root": &project.AccountConfig{
					Address:    types.HexToAddress("01").Hex(),
					PrivateKey: prKeyHex,
				},
			},
		}

		project.SaveConfig(conf)
	},
}
