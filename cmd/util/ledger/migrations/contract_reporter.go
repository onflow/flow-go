package migrations

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// reports on which contracts are deployed
type ContractReporter struct {
	Log       zerolog.Logger
	OutputDir string
}

func (r ContractReporter) filename() string {
	return path.Join(r.OutputDir, fmt.Sprintf("contract_report_%d.csv", int32(time.Now().Unix())))
}

func (r ContractReporter) Report(payload []ledger.Payload) error {
	fn := r.filename()
	r.Log.Info().Msgf("Running Contract Reporter. Saving output to %s.", fn)

	f, err := os.Create(fn)
	if err != nil {
		return err
	}

	defer func() {
		err = f.Close()
		if err != nil {
			panic(err)
		}
	}()

	writer := bufio.NewWriter(f)
	defer func() {
		err = writer.Flush()
		if err != nil {
			panic(err)
		}
	}()

	l := newView(payload)
	st := state.NewState(l, state.NewInteractionLimiter(state.WithInteractionLimit(false)))
	sth := state.NewStateHolder(st)
	accounts := state.NewAccounts(sth)

	for _, p := range payload {
		id, err := keyToRegisterID(p.Key)
		if err != nil {
			return err
		}
		if len([]byte(id.Owner)) != flow.AddressLength {
			// not an address
			continue
		}
		if id.Key != state.KeyContractNames {
			continue
		}
		address := flow.BytesToAddress([]byte(id.Owner))
		contracts, err := accounts.GetContractNames(address)
		if err != nil {
			return err
		}
		if len(contracts) == 0 {
			continue
		}
		for _, contract := range contracts {
			record := fmt.Sprintf("%s,%s\n", address.Hex(), contract)
			_, err = writer.WriteString(record)
			if err != nil {
				return err
			}
		}
	}

	r.Log.Info().Msg("Contract Reporter Done.")

	return nil
}
