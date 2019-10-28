package start

import (
	"fmt"
	"os"
	"time"

	"github.com/dapperlabs/flow-go/internal/cli/initialize"

	"github.com/psiemens/sconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/internal/cli/project"
	"github.com/dapperlabs/flow-go/internal/cli/utils"
	"github.com/dapperlabs/flow-go/pkg/crypto"
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
			pconf := initialize.InitializeProject()
			rootAcct := pconf.RootAccount()

			fmt.Printf("⚙️   Flow client initialized with root account:\n\n")
			fmt.Printf("👤  Address: 0x%s\n", rootAcct.Address)
			fmt.Printf("🔑  PrivateKey: %s\n\n", rootAcct.PrivateKey)
		}
		projectConf := project.LoadConfig()

		if conf.Verbose {
			log.SetLevel(logrus.DebugLevel)
		}

		serverConf := &server.Config{
			Port:           conf.Port,
			HTTPPort:       conf.HTTPPort,
			BlockInterval:  conf.BlockInterval,
			RootAccountKey: getRootKey(projectConf),
		}

		server.StartServer(log, serverConf)
	},
}

func getRootKey(projectConf *project.Config) crypto.PrivateKey {
	if conf.RootKey != "" {
		prKey, err := utils.DecodePrivateKey(conf.RootKey)
		if err != nil {
			fmt.Printf("Failed to decode private key")
			os.Exit(1)
		}

		return prKey
	} else if projectConf != nil {
		rootAccount := projectConf.RootAccount()

		log.Infof("⚙️   Loaded root account key from %s\n", project.ConfigPath)

		return rootAccount.PrivateKey
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
