package start

import (
	"os"
	"time"

	"github.com/psiemens/sconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-node/internal/emulator/server"
	"github.com/dapperlabs/bamboo-node/sdk/accounts"
)

type Config struct {
	Port           int           `default:"5000" flag:"port,p" info:"port to run RPC server"`
	HTTPPort       int           `default:"9090" flag:"http_port" info:"port to run HTTP server"`
	Verbose        bool          `default:"false" flag:"verbose,v" info:"enable verbose logging"`
	BlockInterval  time.Duration `default:"5s" flag:"interval,i" info:"time between minted blocks"`
	RootAccountKey string        `flag:"root,r" info:"path to root account"`
}

var (
	log  *logrus.Logger
	conf Config
)

var Cmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the Bamboo emulator server",
	Run: func(cmd *cobra.Command, args []string) {
		if conf.Verbose {
			log.SetLevel(logrus.DebugLevel)
		}

		serverConf := &server.Config{
			Port:          conf.Port,
			HTTPPort:      conf.HTTPPort,
			BlockInterval: conf.BlockInterval,
		}

		if conf.RootAccountKey != "" {
			accKey, err := accounts.LoadAccountFromFile(conf.RootAccountKey)
			if err != nil {
				log.
					WithError(err).
					Fatalf("Failed to load root account from %s", conf.RootAccountKey)
			}

			serverConf.RootAccountKey = accKey

			log.Infof("Loaded root account from %s", conf.RootAccountKey)
		}

		server.StartServer(log, serverConf)
	},
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
