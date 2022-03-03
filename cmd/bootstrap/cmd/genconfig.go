package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

var (
	flagAddressFormat     string
	flagNodesAccess       int
	flagNodesCollection   int
	flagNodesConsensus    int
	flagNodesExecution    int
	flagNodesVerification int
	// Deprecated: use flagWeight instead
	deprecatedFlagStake uint64
	flagWeight          uint64
)

// genconfigCmdRun generates the node-config.json file
func genconfigCmdRun(_ *cobra.Command, _ []string) {

	// maintain backward compatibility with old flag name
	if deprecatedFlagStake != 0 {
		log.Warn().Msg("using deprecated flag --stake (use --weight instead)")
		if flagWeight == 0 {
			flagWeight = deprecatedFlagStake
		} else {
			log.Fatal().Msg("cannot use both --stake and --weight flags (use only --weight)")
		}
	}

	configs := make([]model.NodeConfig, 0)

	for i := 0; i < flagNodesAccess; i++ {
		configs = append(configs, createConf(flow.RoleAccess, i))
	}
	for i := 0; i < flagNodesCollection; i++ {
		configs = append(configs, createConf(flow.RoleCollection, i))
	}
	for i := 0; i < flagNodesConsensus; i++ {
		configs = append(configs, createConf(flow.RoleConsensus, i))
	}
	for i := 0; i < flagNodesExecution; i++ {
		configs = append(configs, createConf(flow.RoleExecution, i))
	}
	for i := 0; i < flagNodesVerification; i++ {
		configs = append(configs, createConf(flow.RoleVerification, i))
	}

	writeJSON(flagConfig, configs)
}

// genconfigCmd represents the genconfig command
var genconfigCmd = &cobra.Command{
	Use:   "genconfig",
	Short: "Generate node-config.json",
	Long:  "example: go run -tags relic ./cmd/bootstrap genconfig --address-format \"%s-%03d.devnet19.nodes.onflow.org:3569\" --access 2 --collection 3 --consensus 3 --execution 2 --verification 1 --weight 1000",
	Run:   genconfigCmdRun,
}

func init() {
	rootCmd.AddCommand(genconfigCmd)

	genconfigCmd.Flags().StringVar(&flagConfig, "config", "node-config.json",
		"path to output JSON file that will contain generated node configurations (fields Role, Address, Weight)")
	genconfigCmd.Flags().StringVar(&flagAddressFormat, "address-format", "%s-%03d.benchmarknet.nodes.onflow.org:3569",
		"format that should be used for the addresses that are generated, first arg is the role string, second one is the number (e. g. `%v-%03d.benchmarknet.nodes.onflow.org:3569`)")
	genconfigCmd.Flags().IntVar(&flagNodesAccess, "access", 2, "number of access nodes")
	genconfigCmd.Flags().IntVar(&flagNodesCollection, "collection", 3, "number of collection nodes")
	genconfigCmd.Flags().IntVar(&flagNodesConsensus, "consensus", 3, "number of consensus nodes")
	genconfigCmd.Flags().IntVar(&flagNodesExecution, "execution", 2, "number of execution nodes")
	genconfigCmd.Flags().IntVar(&flagNodesVerification, "verification", 1, "number of verification nodes")
	genconfigCmd.Flags().Uint64Var(&flagWeight, "weight", flow.DefaultInitialWeight, "weight for all nodes")
	genconfigCmd.Flags().Uint64Var(&deprecatedFlagStake, "stake", 0, "deprecated: use --weight")
}

func createConf(r flow.Role, i int) model.NodeConfig {
	return model.NodeConfig{
		Role:    r,
		Address: fmt.Sprintf(flagAddressFormat, r.String(), i+1),
		Weight:  flagWeight,
	}
}
