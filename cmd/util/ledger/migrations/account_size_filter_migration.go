package migrations

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
)

func NewAccountSizeFilterMigration(
	maxAccountSize uint64,
	exceptions map[string]struct{},
	log zerolog.Logger,
) RegistersMigration {

	if maxAccountSize == 0 {
		return nil
	}

	return func(registersByAccount *registers.ByAccount) error {

		type accountInfo struct {
			count int
			size  uint64
		}
		payloadCountByAddress := make(map[string]accountInfo)

		err := registersByAccount.ForEach(func(owner string, key string, value []byte) error {

			info := payloadCountByAddress[owner]
			info.count++
			info.size += uint64(len(value))
			payloadCountByAddress[owner] = info

			return nil
		})
		if err != nil {
			return err
		}

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

		return registersByAccount.ForEachAccount(
			func(accountRegisters *registers.AccountRegisters) error {
				owner := accountRegisters.Owner()

				if _, ok := exceptions[owner]; ok {
					return nil
				}

				info := payloadCountByAddress[owner]
				if info.size <= maxAccountSize {
					return nil
				}

				return accountRegisters.ForEach(func(owner, key string, _ []byte) error {
					return accountRegisters.Set(owner, key, nil)
				})
			},
		)
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
