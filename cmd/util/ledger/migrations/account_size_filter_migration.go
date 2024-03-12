package migrations

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
)

func NewAccountSizeFilterMigration(
	maxAccountSize uint64,
	exceptions map[string]struct{},
	log zerolog.Logger,
) ledger.Migration {

	if maxAccountSize == 0 {
		return nil
	}

	return func(payloads []*ledger.Payload) ([]*ledger.Payload, error) {

		type accountInfo struct {
			count int
			size  uint64
		}
		payloadCountByAddress := make(map[string]accountInfo)

		for _, payload := range payloads {
			registerID, payloadValue, err := convert.PayloadToRegister(payload)
			if err != nil {
				return nil, fmt.Errorf("cannot convert payload to register: %w", err)
			}

			owner := registerID.Owner
			accountInfo := payloadCountByAddress[owner]
			accountInfo.count++
			accountInfo.size += uint64(len(payloadValue))
			payloadCountByAddress[owner] = accountInfo
		}

		if log.Debug().Enabled() {
			for address, info := range payloadCountByAddress {
				log.Debug().Msgf(
					"address %x has %d payloads and a total size of %s",
					address,
					info.count,
					ByteCountIEC(int64(info.size)),
				)

				if _, ok := exceptions[address]; !ok && info.size > maxAccountSize {
					log.Warn().Msgf(
						"dropping payloads of account %x. size of payloads %s exceeds max size %s",
						address,
						ByteCountIEC(int64(info.size)),
						ByteCountIEC(int64(maxAccountSize)),
					)
				}
			}
		}

		filteredPayloads := make([]*ledger.Payload, 0, int(0.8*float32(len(payloads))))

		for _, payload := range payloads {
			registerID, _, err := convert.PayloadToRegister(payload)
			if err != nil {
				return nil, fmt.Errorf("cannot convert payload to register: %w", err)
			}

			owner := registerID.Owner

			if _, ok := exceptions[owner]; !ok {
				info := payloadCountByAddress[owner]
				if info.size > maxAccountSize {
					continue
				}
			}

			filteredPayloads = append(filteredPayloads, payload)
		}

		return filteredPayloads, nil
	}
}

func ByteCountIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}
