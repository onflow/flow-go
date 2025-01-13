package checkpoint_collect_stats

import (
	"cmp"
	"slices"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

type AccountStats struct {
	stats
	ServiceAccount *AccountInfo   `json:"service_account,omitempty"`
	EVMAccount     *AccountInfo   `json:"evm_account,omitempty"`
	TopN           []*AccountInfo `json:"largest_accounts"`
}

type AccountInfo struct {
	Address      string `json:"address"`
	PayloadCount uint64 `json:"payload_count"`
	PayloadSize  uint64 `json:"payload_size"`
}

func getAccountStatus(
	chainID flow.ChainID,
	accounts map[string]*AccountInfo,
) AccountStats {
	accountsSlice := make([]*AccountInfo, 0, len(accounts))
	accountSizesSlice := make([]float64, 0, len(accounts))

	for _, acct := range accounts {
		accountsSlice = append(accountsSlice, acct)
		accountSizesSlice = append(accountSizesSlice, float64(acct.PayloadSize))
	}

	// Sort accounts by payload size in descending order
	slices.SortFunc(accountsSlice, func(a, b *AccountInfo) int {
		return cmp.Compare(b.PayloadSize, a.PayloadSize)
	})

	stats := getValueStats(accountSizesSlice, percentiles)

	evmAccountAddress := systemcontracts.SystemContractsForChain(chainID).EVMStorage.Address

	serviceAccountAddress := serviceAccountAddressForChain(chainID)

	return AccountStats{
		stats:          stats,
		ServiceAccount: accounts[string(serviceAccountAddress[:])],
		EVMAccount:     accounts[string(evmAccountAddress[:])],
		TopN:           accountsSlice[:flagTopN],
	}
}

func serviceAccountAddressForChain(chainID flow.ChainID) flow.Address {
	sc := systemcontracts.SystemContractsForChain(chainID)
	return sc.FlowServiceAccount.Address
}
