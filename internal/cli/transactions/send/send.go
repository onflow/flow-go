package send

import (
	"context"
	"io/ioutil"
	"log"

	"github.com/dapperlabs/flow-go/internal/cli/project"
	"github.com/dapperlabs/flow-go/internal/cli/utils"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/client"
	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"
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

		tx := types.Transaction{
			Script:         code,
			Nonce:          conf.Nonce,
			ComputeLimit:   10,
			PayerAccount:   signer.Address,
			ScriptAccounts: []types.Address{signer.Address},
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
