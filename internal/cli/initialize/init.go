package initialize

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
	"github.com/dapperlabs/bamboo-node/sdk/accounts"
)

var Cmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new account profile",
	Run: func(cmd *cobra.Command, args []string) {
		salg, _ := crypto.NewSignatureAlgo(crypto.ECDSA_P256)
		prKey, _ := salg.GeneratePrKey([]byte("elephant ears"))

		accountKey := &types.AccountKey{
			Account: types.HexToAddress("01"),
			Key:     prKey,
		}

		err := accounts.SaveAccountToFile(accountKey, "bamboo.json")
		if err != nil {
			fmt.Printf("failed to write profile to file: %s", err.Error())
		}

		fmt.Println("new profile saved to bamboo.json")
	},
}
