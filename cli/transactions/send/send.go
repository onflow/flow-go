package send

import (
	"context"
	"io/ioutil"
	"log"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cli/project"
	"github.com/dapperlabs/flow-go/cli/utils"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/client"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

type Config struct {
	Signer string `default:"root" flag:"signer,s"`
	Code   string `flag:"code,c"`
	Nonce  uint64 `flag:"nonce,n"`
}

var conf Config

var Cmd = &cobra.Command{
	Use:   "send",
	Short: "Send a transaction",
	Run: func(cmd *cobra.Command, args []string) {
		projectConf := project.LoadConfig()

		signer := projectConf.Accounts[conf.Signer]

		var (
			code []byte
			err  error
		)

		if conf.Code != "" {
			code, err = ioutil.ReadFile(conf.Code)
			if err != nil {
				utils.Exitf(1, "Failed to load BPL code from %s", conf.Code)
			}
		}

		tx := flow.Transaction{
			Script:         code,
			Nonce:          conf.Nonce,
			ComputeLimit:   10,
			PayerAccount:   signer.Address,
			ScriptAccounts: []flow.Address{signer.Address},
		}

		sig, err := keys.SignTransaction(tx, signer.PrivateKey)
		if err != nil {
			utils.Exit(1, "Failed to sign transaction")
		}

		tx.AddSignature(signer.Address, sig)

		client, err := client.New("localhost:5000")
		if err != nil {
			utils.Exit(1, "Failed to connect to emulator")
		}

		err = client.SendTransaction(context.Background(), tx)
		if err != nil {
			utils.Exitf(1, "Failed to send transaction: %v", err)
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
