package migrations

import (
	"bufio"
	"fmt"
	"github.com/onflow/cadence/runtime"
	"os"
	"path"
	"time"

	"github.com/onflow/atree"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/model/flow"
)

// iterates through registers keeping a map of register sizes
// reports on storage metrics
type StorageReporter struct {
	Log         zerolog.Logger
	OutputDir   string
	slabStorage atree.SlabStorage
}

func (r *StorageReporter) filename() string {
	return path.Join(r.OutputDir, fmt.Sprintf("storage_report_%d.csv", int32(time.Now().Unix())))
}

func (r *StorageReporter) Report(payload []ledger.Payload) error {
	fn := r.filename()
	r.Log.Info().Msgf("Running Storage Reporter. Saving output to %s.", fn)

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
	st := state.NewState(l)
	stateHolder := state.NewStateHolder(st)
	accounts := state.NewAccounts(stateHolder)

	r.slabStorage = runtime.NewStorage(
		newAccountsAtreeLedger(accounts),
		func(f func(), _ func(metrics runtime.Metrics, duration time.Duration)) {
			f()
		},
	)

	for _, p := range payload {
		id, err := keyToRegisterID(p.Key)
		if err != nil {
			return err
		}
		if len([]byte(id.Owner)) != flow.AddressLength {
			// not an address
			continue
		}
		if id.Key != "storage_used" {
			continue
		}
		address := flow.BytesToAddress([]byte(id.Owner))
		u, _, err := utils.ReadUint64(p.Value)
		if err != nil {
			return err
		}
		balance, hasVault, err := r.balance(address, st)
		if err != nil {
			r.Log.Err(err).Msg("Cannot get account balance")
			return err
		}
		dapper, err := r.isDapper(address, st)
		if err != nil {
			r.Log.Err(err).Msg("Cannot determine if this is a dapper account")
			return err
		}
		hasReceiver, err := r.hasReceiver(address, st)
		if err != nil {
			r.Log.Err(err).Msg("Cannot determine if this account has a receiver")
			return err
		}
		record := fmt.Sprintf("%s,%d,%d,%t,%t,%t\n", address.Hex(), u, balance, hasVault, hasReceiver, dapper)
		_, err = writer.WriteString(record)
		if err != nil {
			return err
		}
	}

	r.Log.Info().Msg("Storage Reporter Done.")

	return nil
}

func (r *StorageReporter) isDapper(address flow.Address, st *state.State) (bool, error) {
	id := resourceId(address,
		interpreter.PathValue{
			Domain:     common.PathDomainPublic,
			Identifier: "dapperUtilityCoinReceiver",
		})

	receiver, err := st.Get(id.Owner, id.Controller, id.Key)
	if err != nil {
		return false, fmt.Errorf("could not load dapper receiver at %s: %w", address, err)
	}
	return len(receiver) != 0, nil
}

func (r *StorageReporter) hasReceiver(address flow.Address, st *state.State) (bool, error) {
	id := resourceId(address,
		interpreter.PathValue{
			Domain:     common.PathDomainPublic,
			Identifier: "flowTokenReceiver",
		})

	receiver, err := st.Get(id.Owner, id.Controller, id.Key)
	if err != nil {
		return false, fmt.Errorf("could not load receiver at %s: %w", address, err)
	}
	return len(receiver) != 0, nil
}

func (r *StorageReporter) balance(address flow.Address, st *state.State) (balance uint64, hasBalance bool, err error) {
	vaultId := resourceId(address,
		interpreter.PathValue{
			Domain:     common.PathDomainStorage,
			Identifier: "flowTokenVault",
		})

	balanceId := resourceId(address,
		interpreter.PathValue{
			Domain:     common.PathDomainPublic,
			Identifier: "flowTokenBalance",
		})

	balanceCapability, err := st.Get(balanceId.Owner, balanceId.Controller, balanceId.Key)
	if err != nil {
		return 0, false, fmt.Errorf("could not load capability at %s: %w", address, err)
	}

	vaultResource, err := st.Get(vaultId.Owner, vaultId.Controller, vaultId.Key)
	if err != nil {
		return 0, false, fmt.Errorf("could not load resource at %s: %w", address, err)
	}

	if len(vaultResource) == 0 {
		return 0, false, nil
	}

	if len(balanceCapability) == 0 {
		r.Log.Warn().Str("Account", address.HexWithPrefix()).Msgf("Address has a vault, but not a balance capability")
	}
	decoder := interpreter.CBORDecMode.NewByteStreamDecoder(vaultResource)

	storable, err := interpreter.DecodeStorable(decoder, atree.StorageIDUndefined)
	storedValue, err := storable.StoredValue(r.slabStorage)
	value := interpreter.MustConvertStoredValue(storedValue)
	if err != nil {
		return 0, false, fmt.Errorf("could not decode resource at %s: %w", address, err)
	}
	composite, ok := value.(*interpreter.CompositeValue)
	if !ok {
		return 0, false, fmt.Errorf("could not decode composite at %s: %w", address, err)
	}
	balanceField := composite.GetField(nil, nil, "balance")
	balanceValue, ok := balanceField.(interpreter.UFix64Value)
	if !ok {
		return 0, false, fmt.Errorf("could not decode resource at %s: %w", address, err)
	}

	return uint64(balanceValue), true, nil
}

func resourceId(address flow.Address, path interpreter.PathValue) flow.RegisterID {
	// Copied logic from interpreter.storageKey(path)
	key := fmt.Sprintf("%s\x1F%s", path.Domain.Identifier(), path.Identifier)

	return flow.RegisterID{
		Owner:      string(address.Bytes()),
		Controller: "",
		Key:        key,
	}
}
