package reporters

import (
	"fmt"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/model/flow"
)

// AccountReporter iterates through registers keeping a map of register sizes
// reports on storage metrics
type AccountReporter struct {
	Log       zerolog.Logger
	OutputDir string
	RWF       ReportWriterFactory
}

var _ ledger.Reporter = &AccountReporter{}

func (r *AccountReporter) Name() string {
	return "Account Reporter"
}

type accountRecord struct {
	Address        string
	StorageUsed    uint64
	AccountBalance uint64
	HasVault       bool
	HasReceiver    bool
	IsDapper       bool
}

type contractRecord struct {
	Address  string
	Contract string
}

func (r *AccountReporter) Report(payload []ledger.Payload) error {
	rwa := r.RWF.ReportWriter("account_report")
	rwc := r.RWF.ReportWriter("contract_report")
	defer rwa.Close()

	l := migrations.NewView(payload)
	st := state.NewState(l)
	sth := state.NewStateHolder(st)
	accounts := state.NewAccounts(sth)

	for _, p := range payload {
		id, err := migrations.KeyToRegisterID(p.Key)
		if err != nil {
			return err
		}
		if len([]byte(id.Owner)) != flow.AddressLength {
			// not an address
			continue
		}

		switch id.Key {
		case state.KeyStorageUsed:
			err = r.handleStorageUsed(id, p, st, rwa)
		case state.KeyContractNames:
			err = r.handleContractNames(id, accounts, rwc)
		default:
			continue
		}

		if err != nil {
			return err
		}

		if id.Key != "storage_used" {
			continue
		}

	}

	return nil
}

func (r *AccountReporter) isDapper(address flow.Address, st *state.State) (bool, error) {
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

func (r *AccountReporter) hasReceiver(address flow.Address, st *state.State) (bool, error) {
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

func (r *AccountReporter) balance(address flow.Address, st *state.State) (balance uint64, hasBalance bool, err error) {
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

	storedData, version := interpreter.StripMagic(vaultResource)

	commonAddress := common.BytesToAddress([]byte(vaultId.Owner))
	storedValue, err := interpreter.DecodeValue(storedData, &commonAddress, []string{vaultId.Key}, version, nil)
	if err != nil {
		return 0, false, fmt.Errorf("could not decode resource at %s: %w", address, err)
	}
	composite, ok := storedValue.(*interpreter.CompositeValue)
	if !ok {
		return 0, false, fmt.Errorf("could not decode composite at %s: %w", address, err)
	}
	balanceField, ok := composite.Fields().Get("balance")
	if !ok {
		return 0, false, fmt.Errorf("could get balance field at %s: %w", address, err)
	}
	balanceValue, ok := balanceField.(interpreter.UFix64Value)
	if !ok {
		return 0, false, fmt.Errorf("could not decode resource at %s: %w", address, err)
	}

	return uint64(balanceValue), true, nil
}

func (r *AccountReporter) handleStorageUsed(id flow.RegisterID, p ledger.Payload, st *state.State, rwa ReportWriter) error {
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

	rwa.Write(accountRecord{
		Address:        address.Hex(),
		StorageUsed:    u,
		AccountBalance: balance,
		HasVault:       hasVault,
		HasReceiver:    hasReceiver,
		IsDapper:       dapper,
	})

	return nil
}

func (r *AccountReporter) handleContractNames(id flow.RegisterID, accounts *state.Accounts, rwc ReportWriter) error {
	address := flow.BytesToAddress([]byte(id.Owner))
	contracts, err := accounts.GetContractNames(address)
	if err != nil {
		return err
	}
	if len(contracts) == 0 {
		return nil
	}
	for _, contract := range contracts {
		rwc.Write(contractRecord{
			Address:  address.Hex(),
			Contract: contract,
		})
	}
	return nil
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
