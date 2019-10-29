package create

import (
	"context"
	"encoding/hex"
	"io/ioutil"
	"log"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cli/project"
	"github.com/dapperlabs/flow-go/cli/utils"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/client"
	"github.com/dapperlabs/flow-go/sdk/emulator/constants"
	"github.com/dapperlabs/flow-go/sdk/templates"
)

type Config struct {
	Signer string   `default:"root" flag:"signer,s"`
	Keys   []string `flag:"key,k"`
	Code   string   `flag:"code,c" info:"path to a file containing code for the account"`
}

var conf Config

var Cmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new account",
	Run: func(cmd *cobra.Command, args []string) {
		projectConf := project.LoadConfig()

		signer := projectConf.Accounts[conf.Signer]

		accountKeys := make([]flow.AccountPublicKey, len(conf.Keys))

		for i, privateKeyHex := range conf.Keys {
			prKeyDer, err := hex.DecodeString(privateKeyHex)
			if err != nil {
				utils.Exit(1, "Failed to decode private key")
			}

			privateKey, err := flow.DecodeAccountPrivateKey(prKeyDer)
			if err != nil {
				utils.Exit(1, "Failed to decode private key")
			}

			accountKeys[i] = privateKey.PublicKey(constants.AccountKeyWeightThreshold)
		}

		var (
			code []byte
			err  error
		)

		if conf.Code != "" {
			code, err = ioutil.ReadFile(conf.Code)
			if err != nil {
				utils.Exitf(1, "Failed to parse Cadence code from %s", conf.Code)
			}
		}

		script, err := templates.CreateAccount(accountKeys, code)
		if err != nil {
			utils.Exit(1, "Failed to generate transaction script")
		}

		tx := flow.Transaction{
			Script:       script,
			Nonce:        1,
			ComputeLimit: 10,
			PayerAccount: signer.Address,
		}

		err = tx.AddSignature(signer.Address, signer.PrivateKey)
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
		FromEnvironment(utils.EnvPrefix).
		BindFlags(Cmd.PersistentFlags()).
		Parse()
	if err != nil {
		log.Fatal(err)
	}
}
