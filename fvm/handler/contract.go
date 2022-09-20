package handler

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

type sortableContractUpdates struct {
	keys    []programs.ContractUpdateKey
	updates []programs.ContractUpdate
}

func (lists *sortableContractUpdates) Len() int {
	return len(lists.keys)
}

func (lists *sortableContractUpdates) Swap(i, j int) {
	lists.keys[i], lists.keys[j] = lists.keys[j], lists.keys[i]
	lists.updates[i], lists.updates[j] = lists.updates[j], lists.updates[i]
}

func (lists *sortableContractUpdates) Less(i, j int) bool {
	switch bytes.Compare(lists.keys[i].Address[:], lists.keys[j].Address[:]) {
	case -1:
		return true
	case 0:
		return lists.keys[i].Name < lists.keys[j].Name
	default:
		return false
	}
}

// ContractUpdater handles all smart contracts modification. It also captures
// all changes as deltas and only commit them when called so smart contract
// updates can be delayed until end of the tx execution.
//
// Note that scripts cannot modify smart contracts, but must expose the API in
// compliance with the runtime environment interface.
type ContractUpdater interface {
	UpdateAccountContractCode(
		address runtime.Address,
		name string,
		code []byte,
	) error

	RemoveAccountContractCode(address runtime.Address, name string) error

	Commit() ([]programs.ContractUpdateKey, error)

	Reset()
}

type NoContractUpdater struct{}

func (NoContractUpdater) UpdateAccountContractCode(
	address runtime.Address,
	name string,
	code []byte,
) error {
	return errors.NewOperationNotSupportedError("UpdateAccountContractCode")
}

func (NoContractUpdater) RemoveAccountContractCode(
	address runtime.Address,
	name string,
) error {
	return errors.NewOperationNotSupportedError("RemoveAccountContractCode")
}

func (NoContractUpdater) Commit() ([]programs.ContractUpdateKey, error) {
	return nil, nil
}

func (NoContractUpdater) Reset() {
}

// Expose stub interface for testing.
type ContractUpdaterStubs interface {
	RestrictedDeploymentEnabled() bool
	RestrictedRemovalEnabled() bool

	GetAuthorizedAccounts(path cadence.Path) []common.Address

	UseContractAuditVoucher(address runtime.Address, code []byte) (bool, error)
}

type contractUpdaterStubsImpl struct {
	chain                      flow.Chain
	restrictContractDeployment bool
	restrictContractRemoval    bool

	logger          *environment.ProgramLogger
	systemContracts *environment.SystemContracts
	runtime         *environment.Runtime
}

func (impl *contractUpdaterStubsImpl) RestrictedDeploymentEnabled() bool {
	enabled, defined := impl.getIsContractDeploymentRestricted()
	if !defined {
		// If the contract deployment bool is not set by the state
		// fallback to the default value set by the configuration
		// after the contract deployment bool is set by the state on all
		// chains, this logic can be simplified
		return impl.restrictContractDeployment
	}
	return enabled
}

// GetIsContractDeploymentRestricted returns if contract deployment
// restriction is defined in the state and the value of it
func (impl *contractUpdaterStubsImpl) getIsContractDeploymentRestricted() (
	restricted bool,
	defined bool,
) {
	service := runtime.Address(impl.chain.ServiceAddress())

	runtime := impl.runtime.BorrowCadenceRuntime()
	defer impl.runtime.ReturnCadenceRuntime(runtime)

	value, err := runtime.ReadStored(
		service,
		blueprints.IsContractDeploymentRestrictedPath)
	if err != nil {
		impl.logger.Logger().
			Debug().
			Msg("Failed to read IsContractDeploymentRestricted from the " +
				"service account. Using value from context instead.")
		return false, false
	}
	restrictedCadence, ok := value.(cadence.Bool)
	if !ok {
		impl.logger.Logger().
			Debug().
			Msg("Failed to parse IsContractDeploymentRestricted from the " +
				"service account. Using value from context instead.")
		return false, false
	}
	restricted = restrictedCadence.ToGoValue().(bool)
	return restricted, true
}

func (impl *contractUpdaterStubsImpl) RestrictedRemovalEnabled() bool {
	// TODO read this from the chain similar to the contract deployment
	// but for now we would honor the fallback context flag
	return impl.restrictContractRemoval
}

// GetAuthorizedAccounts returns a list of addresses authorized by the service
// account. Used to determine which accounts are permitted to deploy, update,
// or remove contracts.
//
// It reads a storage path from service account and parse the addresses. If any
// issue occurs on the process (missing registers, stored value properly not
// set), it gracefully handles it and falls back to default behaviour (only
// service account be authorized).
func (impl *contractUpdaterStubsImpl) GetAuthorizedAccounts(
	path cadence.Path,
) []common.Address {
	// set default to service account only
	service := runtime.Address(impl.chain.ServiceAddress())
	defaultAccounts := []runtime.Address{service}

	runtime := impl.runtime.BorrowCadenceRuntime()
	defer impl.runtime.ReturnCadenceRuntime(runtime)

	value, err := runtime.ReadStored(service, path)

	const warningMsg = "failed to read contract authorized accounts from " +
		"service account. using default behaviour instead."

	if err != nil {
		impl.logger.Logger().Warn().Msg(warningMsg)
		return defaultAccounts
	}
	addresses, ok := utils.CadenceValueToAddressSlice(value)
	if !ok {
		impl.logger.Logger().Warn().Msg(warningMsg)
		return defaultAccounts
	}
	return addresses
}

func (impl *contractUpdaterStubsImpl) UseContractAuditVoucher(
	address runtime.Address,
	code []byte,
) (
	bool,
	error,
) {
	return impl.systemContracts.UseContractAuditVoucher(
		address,
		string(code[:]))
}

type contractUpdater struct {
	tracer          *environment.Tracer
	meter           environment.Meter
	accounts        environment.Accounts
	transactionInfo environment.TransactionInfo

	draftUpdates map[programs.ContractUpdateKey]programs.ContractUpdate

	ContractUpdaterStubs
}

var _ ContractUpdater = &contractUpdater{}

func NewContractUpdaterForTesting(
	accounts environment.Accounts,
	stubs ContractUpdaterStubs,
) *contractUpdater {
	updater := NewContractUpdater(
		nil,
		nil,
		accounts,
		nil,
		nil,
		false,
		false,
		nil,
		nil,
		nil)
	updater.ContractUpdaterStubs = stubs
	return updater
}

func NewContractUpdater(
	tracer *environment.Tracer,
	meter environment.Meter,
	accounts environment.Accounts,
	transactionInfo environment.TransactionInfo,
	chain flow.Chain,
	restrictContractDeployment bool,
	restrictContractRemoval bool,
	logger *environment.ProgramLogger,
	systemContracts *environment.SystemContracts,
	runtime *environment.Runtime,
) *contractUpdater {
	updater := &contractUpdater{
		tracer:          tracer,
		meter:           meter,
		accounts:        accounts,
		transactionInfo: transactionInfo,
		ContractUpdaterStubs: &contractUpdaterStubsImpl{
			logger:                     logger,
			chain:                      chain,
			restrictContractDeployment: restrictContractDeployment,
			restrictContractRemoval:    restrictContractRemoval,
			systemContracts:            systemContracts,
			runtime:                    runtime,
		},
	}

	updater.Reset()
	return updater
}

func (updater *contractUpdater) UpdateAccountContractCode(
	address runtime.Address,
	name string,
	code []byte,
) error {
	defer updater.tracer.StartSpanFromRoot(
		trace.FVMEnvUpdateAccountContractCode).End()

	err := updater.meter.MeterComputation(
		environment.ComputationKindUpdateAccountContractCode,
		1)
	if err != nil {
		return fmt.Errorf("update account contract code failed: %w", err)
	}

	err = updater.accounts.CheckAccountNotFrozen(flow.Address(address))
	if err != nil {
		return fmt.Errorf("update account contract code failed: %w", err)
	}

	err = updater.setContract(
		address,
		name,
		code,
		updater.transactionInfo.SigningAccounts())
	if err != nil {
		return fmt.Errorf("updating account contract code failed: %w", err)
	}

	return nil
}

func (updater *contractUpdater) RemoveAccountContractCode(
	address runtime.Address,
	name string,
) error {
	defer updater.tracer.StartSpanFromRoot(
		trace.FVMEnvRemoveAccountContractCode).End()

	err := updater.meter.MeterComputation(
		environment.ComputationKindRemoveAccountContractCode,
		1)
	if err != nil {
		return fmt.Errorf("remove account contract code failed: %w", err)
	}

	err = updater.accounts.CheckAccountNotFrozen(flow.Address(address))
	if err != nil {
		return fmt.Errorf("remove account contract code failed: %w", err)
	}

	err = updater.removeContract(
		address,
		name,
		updater.transactionInfo.SigningAccounts())
	if err != nil {
		return fmt.Errorf("remove account contract code failed: %w", err)
	}

	return nil
}

func (updater *contractUpdater) setContract(
	address runtime.Address,
	name string,
	code []byte,
	signingAccounts []runtime.Address,
) (err error) {

	flowAddress := flow.Address(address)

	// Initial contract deployments must be authorized by signing accounts,
	// or there must be an audit voucher available.
	//
	// Contract updates are always allowed.

	var exists bool
	exists, err = updater.accounts.ContractExists(name, flowAddress)
	if err != nil {
		return err
	}

	if !exists && !updater.isAuthorizedForDeployment(signingAccounts) {
		// check if there's an audit voucher for the contract
		voucherAvailable, err := updater.UseContractAuditVoucher(address, code)
		if err != nil {
			errInner := errors.NewOperationAuthorizationErrorf(
				"SetContract",
				"failed to check audit vouchers",
			)
			return fmt.Errorf("setting contract failed: %w - %s", errInner, err)
		}
		if !voucherAvailable {
			return fmt.Errorf(
				"deploying contract failed: %w",
				errors.NewOperationAuthorizationErrorf(
					"SetContract",
					"deploying contracts requires authorization from specific "+
						"accounts"))
		}
	}

	contractUpdateKey := programs.ContractUpdateKey{
		Address: flowAddress,
		Name:    name,
	}

	updater.draftUpdates[contractUpdateKey] = programs.ContractUpdate{
		ContractUpdateKey: contractUpdateKey,
		Code:              code,
	}

	return nil
}

func (updater *contractUpdater) removeContract(
	address runtime.Address,
	name string,
	signingAccounts []runtime.Address,
) (err error) {
	// check if authorized
	if !updater.isAuthorizedForRemoval(signingAccounts) {
		return fmt.Errorf("removing contract failed: %w",
			errors.NewOperationAuthorizationErrorf(
				"RemoveContract",
				"removing contracts requires authorization from specific "+
					"accounts"))
	}

	add := flow.Address(address)
	uk := programs.ContractUpdateKey{Address: add, Name: name}
	u := programs.ContractUpdate{ContractUpdateKey: uk}
	updater.draftUpdates[uk] = u

	return nil
}

func (updater *contractUpdater) Commit() ([]programs.ContractUpdateKey, error) {
	updatedKeys, updateList := updater.updates()
	updater.Reset()

	var err error
	for _, v := range updateList {
		if len(v.Code) > 0 {
			err = updater.accounts.SetContract(v.Name, v.Address, v.Code)
			if err != nil {
				return nil, err
			}
		} else {
			err = updater.accounts.DeleteContract(v.Name, v.Address)
			if err != nil {
				return nil, err
			}
		}
	}

	return updatedKeys, nil
}

func (updater *contractUpdater) Reset() {
	updater.draftUpdates = make(map[programs.ContractUpdateKey]programs.ContractUpdate)
}

func (updater *contractUpdater) hasUpdates() bool {
	return len(updater.draftUpdates) > 0
}

func (updater *contractUpdater) updates() (
	[]programs.ContractUpdateKey,
	[]programs.ContractUpdate,
) {
	if len(updater.draftUpdates) == 0 {
		return nil, nil
	}
	keys := make([]programs.ContractUpdateKey, 0, len(updater.draftUpdates))
	updates := make([]programs.ContractUpdate, 0, len(updater.draftUpdates))
	for key, update := range updater.draftUpdates {
		keys = append(keys, key)
		updates = append(updates, update)
	}

	sort.Sort(&sortableContractUpdates{keys: keys, updates: updates})
	return keys, updates
}

func (updater *contractUpdater) isAuthorizedForDeployment(
	signingAccounts []runtime.Address,
) bool {
	if updater.RestrictedDeploymentEnabled() {
		return updater.isAuthorized(
			signingAccounts,
			blueprints.ContractDeploymentAuthorizedAddressesPath)
	}
	return true
}

func (updater *contractUpdater) isAuthorizedForRemoval(
	signingAccounts []runtime.Address,
) bool {
	if updater.RestrictedRemovalEnabled() {
		return updater.isAuthorized(
			signingAccounts,
			blueprints.ContractRemovalAuthorizedAddressesPath)
	}
	return true
}

func (updater *contractUpdater) isAuthorized(
	signingAccounts []runtime.Address,
	path cadence.Path,
) bool {
	accts := updater.GetAuthorizedAccounts(path)
	for _, authorized := range accts {
		for _, signer := range signingAccounts {
			if signer == authorized {
				// a single authorized singer is enough
				return true
			}
		}
	}
	return false
}
