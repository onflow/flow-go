package checkpoint_collect_stats

import (
	"cmp"
	"slices"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

type accountFormat uint8

const (
	accountFormatUnknown accountFormat = iota
	accountFormatV1
	accountFormatV2
)

func (format accountFormat) MarshalJSON() ([]byte, error) {
	switch format {
	case accountFormatV1:
		return []byte("\"v1\""), nil

	case accountFormatV2:
		return []byte("\"v2\""), nil

	default:
		return []byte("\"unknown\""), nil
	}
}

type AccountStats struct {
	stats
	FormatV1Count  int            `json:"account_format_v1_count"`
	FormatV2Count  int            `json:"account_format_v2_count"`
	ServiceAccount *AccountInfo   `json:"service_account,omitempty"`
	EVMAccount     *AccountInfo   `json:"evm_account,omitempty"`
	TopN           []*AccountInfo `json:"largest_accounts"`
}

type AccountInfo struct {
	Address      string        `json:"address"`
	Format       accountFormat `json:"account_format"`
	PayloadCount uint64        `json:"payload_count"`
	PayloadSize  uint64        `json:"payload_size"`
}

func getAccountStatus(
	chainID flow.ChainID,
	accounts map[string]*AccountInfo,
) AccountStats {
	accountsSlice := make([]*AccountInfo, 0, len(accounts))
	accountSizesSlice := make([]float64, 0, len(accounts))

	var accountFormatV1Count, accountFormatV2Count int

	for _, acct := range accounts {
		accountsSlice = append(accountsSlice, acct)
		accountSizesSlice = append(accountSizesSlice, float64(acct.PayloadSize))

		switch acct.Format {
		case accountFormatV1:
			accountFormatV1Count++

		case accountFormatV2:
			accountFormatV2Count++

		default:
			if acct.Address != "" {
				log.Info().Msgf("found account without account register nor domain register: %x", acct.Address)
			}
		}
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
		FormatV1Count:  accountFormatV1Count,
		FormatV2Count:  accountFormatV2Count,
		ServiceAccount: accounts[string(serviceAccountAddress[:])],
		EVMAccount:     accounts[string(evmAccountAddress[:])],
		TopN:           accountsSlice[:flagTopN],
	}
}

func serviceAccountAddressForChain(chainID flow.ChainID) flow.Address {
	sc := systemcontracts.SystemContractsForChain(chainID)
	return sc.FlowServiceAccount.Address
}
