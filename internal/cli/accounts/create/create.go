package create

import (
	"context"
	"io/ioutil"
	"log"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/internal/cli/project"
	"github.com/dapperlabs/flow-go/internal/cli/utils"
	"github.com/dapperlabs/flow-go/pkg/constants"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/client"
	"github.com/dapperlabs/flow-go/sdk/templates"
)

type Config struct {
	Signer string   `default:"root" flag:"signer,s"`
	Keys   []string `flag:"key,k"`
	Code   string   `flag:"code,c"`
}

var conf Config

var Cmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new account",
	Run: func(cmd *cobra.Command, args []string) {
		projectConf := project.LoadConfig()

		signer := projectConf.Accounts[conf.Signer]
		signerAddr := types.HexToAddress(signer.Address)
		signerKey, err := utils.DecodePrivateKey(signer.PrivateKey)
		if err != nil {
			utils.Exit(1, "Failed to load signer key")
		}

		accountKeys := make([]types.AccountKey, len(conf.Keys))

		for i, privateKeyStr := range conf.Keys {
			privateKey, err := utils.DecodePrivateKey(privateKeyStr)
			if err != nil {
				utils.Exit(1, "Failed to decode private key")
			}

			publicKey, err := utils.EncodePublicKey(privateKey.PublicKey())
			if err != nil {
				utils.Exit(1, "Failed to encode public key")
			}

			accountKeys[i] = types.AccountKey{
				PublicKey: publicKey,
				Weight:    constants.AccountKeyWeightThreshold,
			}
		}

		var code []byte

		if conf.Code != "" {
			code, err = ioutil.ReadFile(conf.Code)
			if err != nil {
				utils.Exitf(1, "Failed to parse Cadence code from %s", conf.Code)
			}
		}

		script := templates.CreateAccount(accountKeys, code)

		tx := types.Transaction{
			Script:       script,
			Nonce:        1,
			ComputeLimit: 10,
			PayerAccount: signerAddr,
		}

		err = tx.AddSignature(signerAddr, signerKey)
		if err != nil {
			utils.Exit(1, "Failed to sign transaction")
		}

		client, err := client.New("localhost:5000")
		if err != nil {
			utils.Exit(1, "Failed to connect to emulator")
		}

		err = client.SendTransaction(context.Background(), tx)
		if err != nil {
			utils.Exit(1, "Failed to send account creation transaction")
		}
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
