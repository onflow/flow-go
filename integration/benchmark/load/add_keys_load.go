package load

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/integration/benchmark/account"
)

type AddKeysLoad struct {
	numberOfKeysToAdd int
}

func NewAddKeysLoad() *AddKeysLoad {
	return &AddKeysLoad{
		10,
	}
}

var _ Load = (*AddKeysLoad)(nil)

func (l *AddKeysLoad) Type() LoadType {
	return AddKeysLoadType
}

func (l *AddKeysLoad) Setup(_ zerolog.Logger, _ LoadContext) error {
	return nil
}

func (l *AddKeysLoad) Load(log zerolog.Logger, lc LoadContext) error {
	wrapErr := func(err error) error {
		return fmt.Errorf("failed to send load: %w", err)
	}

	acc, err := lc.BorrowAvailableAccount()
	if err != nil {
		return wrapErr(err)
	}
	defer lc.ReturnAvailableAccount(acc)

	return account.AddKeysToAccount(log, acc, l.numberOfKeysToAdd, lc, lc)
}
