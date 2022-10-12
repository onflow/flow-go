package environment

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

// AccountFreezer disables accounts.
//
// Note that scripts cannot freeze accounts, but must expose the API in
// compliance with the environment interface.
type AccountFreezer interface {
	// Note that the script variant will return OperationNotSupportedError.
	SetAccountFrozen(address common.Address, frozen bool) error

	FrozenAccounts() []common.Address

	Reset()
}

type ParseRestrictedAccountFreezer struct {
	txnState *state.TransactionState
	impl     AccountFreezer
}

func NewParseRestrictedAccountFreezer(
	txnState *state.TransactionState,
	impl AccountFreezer,
) AccountFreezer {
	return ParseRestrictedAccountFreezer{
		txnState: txnState,
		impl:     impl,
	}
}

func (freezer ParseRestrictedAccountFreezer) SetAccountFrozen(
	address common.Address,
	frozen bool,
) error {
	return parseRestrict2Arg(
		freezer.txnState,
		"SetAccountFrozen",
		freezer.impl.SetAccountFrozen,
		address,
		frozen)
}

func (freezer ParseRestrictedAccountFreezer) FrozenAccounts() []common.Address {
	return freezer.impl.FrozenAccounts()
}

func (freezer ParseRestrictedAccountFreezer) Reset() {
	freezer.impl.Reset()
}

type NoAccountFreezer struct{}

func (NoAccountFreezer) FrozenAccounts() []common.Address {
	return nil
}

func (NoAccountFreezer) SetAccountFrozen(_ common.Address, _ bool) error {
	return errors.NewOperationNotSupportedError("SetAccountFrozen")
}

func (NoAccountFreezer) Reset() {
}

type accountFreezer struct {
	serviceAddress flow.Address

	accounts        Accounts
	transactionInfo TransactionInfo

	frozenAccounts []common.Address
}

func NewAccountFreezer(
	serviceAddress flow.Address,
	accounts Accounts,
	transactionInfo TransactionInfo,
) *accountFreezer {
	freezer := &accountFreezer{
		serviceAddress:  serviceAddress,
		accounts:        accounts,
		transactionInfo: transactionInfo,
	}
	freezer.Reset()
	return freezer
}

func (freezer *accountFreezer) Reset() {
	freezer.frozenAccounts = nil
}

func (freezer *accountFreezer) FrozenAccounts() []common.Address {
	return freezer.frozenAccounts
}

func (freezer *accountFreezer) SetAccountFrozen(
	address common.Address,
	frozen bool,
) error {
	flowAddress := flow.Address(address)

	if flowAddress == freezer.serviceAddress {
		return fmt.Errorf(
			"setting account frozen failed: %w",
			errors.NewValueErrorf(
				flowAddress.String(),
				"cannot freeze service account"))
	}

	if !freezer.transactionInfo.IsServiceAccountAuthorizer() {
		return fmt.Errorf(
			"setting account frozen failed: %w",
			errors.NewOperationAuthorizationErrorf(
				"SetAccountFrozen",
				"accounts can be frozen only by transactions authorized by "+
					"the service account"))
	}

	err := freezer.accounts.SetAccountFrozen(flowAddress, frozen)
	if err != nil {
		return fmt.Errorf("setting account frozen failed: %w", err)
	}

	if frozen {
		freezer.frozenAccounts = append(freezer.frozenAccounts, address)
	}

	return nil
}
