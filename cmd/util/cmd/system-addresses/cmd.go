package addresses

import (
	"bytes"
	"sort"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

var (
	flagChain     string
	flagSeparator string
)

var Cmd = &cobra.Command{
	Use:   "system-addresses",
	Short: "print addresses of system contracts",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagChain, "chain", "", "Chain name")
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().StringVar(&flagSeparator, "separator", ",", "Separator to use between addresses")
}

func run(*cobra.Command, []string) {
	chainID := flow.ChainID(flagChain)
	// validate
	_ = chainID.Chain()

	systemContracts := systemcontracts.SystemContractsForChain(chainID)

	addressSet := map[flow.Address]struct{}{}
	for _, contract := range systemContracts.All() {
		addressSet[contract.Address] = struct{}{}
	}

	addresses := make([]flow.Address, 0, len(addressSet))
	for address := range addressSet {
		addresses = append(addresses, address)
	}

	sort.Slice(addresses, func(i, j int) bool {
		a := addresses[i]
		b := addresses[j]
		return bytes.Compare(a[:], b[:]) < 0
	})

	for i, address := range addresses {
		str := address.Hex()

		if i > 0 {
			print(flagSeparator)
		}
		print(str)
	}
}
