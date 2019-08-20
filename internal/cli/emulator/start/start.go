package start

import (
	"os"
	"time"

	"github.com/psiemens/sconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-node/internal/emulator/server"
)

type Config struct {
	Port          int           `default:"5000" flag:"port,p" info:"port to run RPC server"`
	HTTPPort      int           `default:"9090" flag:"http_port" info:"port to run HTTP server"`
	BlockInterval time.Duration `default:"5s" flag:"interval,i" info:"time between minted blocks"`
}

var (
	log  *logrus.Logger
	conf Config
)

var Cmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the Bamboo emulator server",
	Run: func(cmd *cobra.Command, args []string) {
		server.StartServer(log, &server.Config{
			Port:          conf.Port,
			HTTPPort:      conf.HTTPPort,
			BlockInterval: conf.BlockInterval,
		})
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
