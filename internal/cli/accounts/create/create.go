package create

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/internal/cli/project"
	"github.com/dapperlabs/flow-go/internal/cli/utils"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/accounts"
	"github.com/dapperlabs/flow-go/sdk/client"
)

type Config struct {
	Signer string `default:"root" flag:"signer,s"`
	Key    string `flag:"key,k"`
	Code   string `flag:"code,c"`
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
			utils.Exit("Failed to load signer key", 1)
		}

		var publicKey []byte
		var code []byte

		if conf.Key != "" {
			accountKey, err := utils.DecodePrivateKey(conf.Key)
			if err != nil {
				utils.Exit("Failed to decode private key", 1)
			}

			publicKey, err = utils.EncodePublicKey(accountKey.Publickey())
			if err != nil {
				utils.Exit("Failed to encode public key", 1)
			}
		}

		if conf.Code != "" {
			code, err = ioutil.ReadFile(conf.Code)
			if err != nil {
				utils.Exit(fmt.Sprintf("Failed to load BPL code from %s", conf.Code), 1)
			}
		}

		script := accounts.CreateAccount(publicKey, code)

		tx := types.Transaction{
			Script:       script,
			ComputeLimit: 10,
			PayerAccount: signerAddr,
		}

		err = tx.AddSignature(signerAddr, signerKey)
		if err != nil {
			utils.Exit("Failed to sign transaction", 1)
		}

		client, err := client.New("localhost:5000")
		if err != nil {
			utils.Exit("Failed to connect to emulator", 1)
		}

		err = client.SendTransaction(context.Background(), tx)
		if err != nil {
			utils.Exit("Failed to send account creation transaction", 1)
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
