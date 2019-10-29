package start

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/psiemens/sconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cli/initialize"
	"github.com/dapperlabs/flow-go/cli/project"
	"github.com/dapperlabs/flow-go/cli/utils"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator/server"
)

type Config struct {
	Port          int           `default:"5000" flag:"port,p" info:"port to run RPC server"`
	HTTPPort      int           `default:"9090" flag:"http_port" info:"port to run HTTP server"`
	Verbose       bool          `default:"false" flag:"verbose,v" info:"enable verbose logging"`
	BlockInterval time.Duration `default:"5s" flag:"interval,i" info:"time between minted blocks"`
	RootKey       string        `flag:"root-key" info:"root account key"`
	Init          bool          `default:"false" flag:"init" info:"whether to initialize a new account profile"`
}

var (
	log  *logrus.Logger
	conf Config
)

var Cmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the Flow emulator server",
	Run: func(cmd *cobra.Command, args []string) {
		if conf.Init {
			pconf := initialize.InitProject()
			rootAcct := pconf.RootAccount()

			fmt.Printf("⚙️   Flow client initialized with root account:\n\n")
			fmt.Printf("👤  Address: 0x%s\n", rootAcct.Address)
		}

		projectConf := project.LoadConfig()

		if conf.Verbose {
			log.SetLevel(logrus.DebugLevel)
		}

		serverConf := &server.Config{
			Port:           conf.Port,
			HTTPPort:       conf.HTTPPort,
			BlockInterval:  conf.BlockInterval,
			RootAccountKey: getRootPrivateKey(projectConf),
		}

		server.StartServer(log, serverConf)
	},
}

func getRootPrivateKey(projectConf *project.Config) *flow.AccountPrivateKey {
	if conf.RootKey != "" {
		prKeyDer, err := hex.DecodeString(conf.RootKey)
		privateKey, err := flow.DecodeAccountPrivateKey(prKeyDer)
		if err != nil {
			utils.Exitf(1, "Failed to decode private key: %v", err)
		}

		return &privateKey
	} else if projectConf != nil {
		rootAccount := projectConf.RootAccount()

		log.Infof("⚙️   Loaded root account key from %s\n", project.ConfigPath)

		return &rootAccount.PrivateKey
	}

	log.Warnf("⚙️   No project configured, generating new root account key")
	return nil
}

func init() {
	initLogger()
	initConfig()
}

func initLogger() {
	log = logrus.New()
	log.Formatter = new(logrus.TextFormatter)
	log.Out = os.Stdout
}

func initConfig() {
	err := sconfig.New(&conf).
		FromEnvironment("BAM").
		BindFlags(Cmd.PersistentFlags()).
		Parse()
	if err != nil {
		log.Fatal(err)
	}
}
