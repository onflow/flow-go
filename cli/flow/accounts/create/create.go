package create

import (
	"context"
	"io/ioutil"
	"log"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cli"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/client"
	"github.com/dapperlabs/flow-go/sdk/keys"
	"github.com/dapperlabs/flow-go/sdk/templates"
)

type Config struct {
	Signer string   `default:"root" flag:"signer,s"`
	Keys   []string `flag:"key,k"`
	Code   string   `flag:"code,c" info:"path to a file containing code for the account"`
	Host   string   `default:"127.0.0.1:3569" flag:"host" info:"Flow Observation API host address"`
}

var conf Config

var Cmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new account",
	Run: func(cmd *cobra.Command, args []string) {
		projectConf := cli.LoadConfig()

		signer := projectConf.Accounts[conf.Signer]

		accountKeys := make([]flow.AccountPublicKey, len(conf.Keys))

		for i, privateKeyHex := range conf.Keys {
			privateKey := cli.MustDecodeAccountPrivateKeyHex(privateKeyHex)
			accountKeys[i] = privateKey.PublicKey(keys.PublicKeyWeightThreshold)
		}

		var (
			code []byte
			err  error
		)

		if conf.Code != "" {
			code, err = ioutil.ReadFile(conf.Code)
			if err != nil {
				cli.Exitf(1, "Failed to parse Cadence code from %s", conf.Code)
			}
		}

		script, err := templates.CreateAccount(accountKeys, code)
		if err != nil {
			cli.Exit(1, "Failed to generate transaction script")
		}

		tx := flow.Transaction{
			Script:       script,
			Nonce:        1,
			ComputeLimit: 10,
			PayerAccount: signer.Address,
		}

		sig, err := keys.SignTransaction(tx, signer.PrivateKey)
		if err != nil {
			cli.Exit(1, "Failed to sign transaction")
		}

		tx.AddSignature(signer.Address, sig)

		client, err := client.New(conf.Host)
		if err != nil {
			cli.Exit(1, "Failed to connect to emulator")
		}

		err = client.SendTransaction(context.Background(), tx)
		if err != nil {
			cli.Exit(1, "Failed to send account creation transaction")
		}
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
