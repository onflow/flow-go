package environment

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

type ContractUpdaterParams struct {
	// Depricated: RestrictedDeploymentEnabled is deprecated use
	// SetIsContractDeploymentRestrictedTransaction instead.
	// Can be removed after all networks are migrated to
	// SetIsContractDeploymentRestrictedTransaction
	RestrictContractDeployment bool
	RestrictContractRemoval    bool
}

func DefaultContractUpdaterParams() ContractUpdaterParams {
	return ContractUpdaterParams{
		RestrictContractDeployment: true,
		RestrictContractRemoval:    true,
	}
}

type sortableContractUpdates struct {
	keys    []common.AddressLocation
	updates []ContractUpdate
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

// ContractUpdater handles all smart contracts modification. It captures
// contract updates and defer the updates to the end of the txn execution.
//
// Note that scripts cannot modify smart contracts, but must expose the API in
// compliance with the runtime environment interface.
type ContractUpdater interface {
	// Cadence's runtime API.  Note that the script variant will return
	// OperationNotSupportedError.
	UpdateAccountContractCode(
		location common.AddressLocation,
		code []byte,
	) error

	// Cadence's runtime API.  Note that the script variant will return
	// OperationNotSupportedError.
	RemoveAccountContractCode(location common.AddressLocation) error

	Commit() (ContractUpdates, error)

	Reset()
}

type ParseRestrictedContractUpdater struct {
	txnState state.NestedTransactionPreparer
	impl     ContractUpdater
}

func NewParseRestrictedContractUpdater(
	txnState state.NestedTransactionPreparer,
	impl ContractUpdater,
) ParseRestrictedContractUpdater {
	return ParseRestrictedContractUpdater{
		txnState: txnState,
		impl:     impl,
	}
}

func (updater ParseRestrictedContractUpdater) UpdateAccountContractCode(
	location common.AddressLocation,
	code []byte,
) error {
	return parseRestrict2Arg(
		updater.txnState,
		trace.FVMEnvUpdateAccountContractCode,
		updater.impl.UpdateAccountContractCode,
		location,
		code)
}

func (updater ParseRestrictedContractUpdater) RemoveAccountContractCode(
	location common.AddressLocation,
) error {
	return parseRestrict1Arg(
		updater.txnState,
		trace.FVMEnvRemoveAccountContractCode,
		updater.impl.RemoveAccountContractCode,
		location)
}

func (updater ParseRestrictedContractUpdater) Commit() (
	ContractUpdates,
	error,
) {
	return updater.impl.Commit()
}

func (updater ParseRestrictedContractUpdater) Reset() {
	updater.impl.Reset()
}

type NoContractUpdater struct{}

func (NoContractUpdater) UpdateAccountContractCode(
	_ common.AddressLocation,
	_ []byte,
) error {
	return errors.NewOperationNotSupportedError("UpdateAccountContractCode")
}

func (NoContractUpdater) RemoveAccountContractCode(
	_ common.AddressLocation,
) error {
	return errors.NewOperationNotSupportedError("RemoveAccountContractCode")
}

func (NoContractUpdater) Commit() (ContractUpdates, error) {
	return ContractUpdates{}, nil
}

func (NoContractUpdater) Reset() {
}

// Expose stub interface for testing.
type ContractUpdaterStubs interface {
	RestrictedDeploymentEnabled() bool
	RestrictedRemovalEnabled() bool

	GetAuthorizedAccounts(path cadence.Path) []flow.Address
}

type contractUpdaterStubsImpl struct {
	chain flow.Chain

	ContractUpdaterParams

	logger          *ProgramLogger
	systemContracts *SystemContracts
	runtime         *Runtime
}

func (impl *contractUpdaterStubsImpl) RestrictedDeploymentEnabled() bool {
	enabled, defined := impl.getIsContractDeploymentRestricted()
	if !defined {
		// If the contract deployment bool is not set by the state
		// fallback to the default value set by the configuration
		// after the contract deployment bool is set by the state on all
		// chains, this logic can be simplified
		return impl.RestrictContractDeployment
	}
	return enabled
}

// GetIsContractDeploymentRestricted returns if contract deployment
// restriction is defined in the state and the value of it
func (impl *contractUpdaterStubsImpl) getIsContractDeploymentRestricted() (
	restricted bool,
	defined bool,
) {
	service := impl.chain.ServiceAddress()

	runtime := impl.runtime.BorrowCadenceRuntime()
	defer impl.runtime.ReturnCadenceRuntime(runtime)

	value, err := runtime.ReadStored(
		common.MustBytesToAddress(service.Bytes()),
		blueprints.IsContractDeploymentRestrictedPath)
	if err != nil {
		impl.logger.
			Debug().
			Msg("Failed to read IsContractDeploymentRestricted from the " +
				"service account. Using value from context instead.")
		return false, false
	}
	restrictedCadence, ok := value.(cadence.Bool)
	if !ok {
		impl.logger.
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
	return impl.RestrictContractRemoval
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
) []flow.Address {
	// set default to service account only
	service := impl.chain.ServiceAddress()
	defaultAccounts := []flow.Address{service}

	runtime := impl.runtime.BorrowCadenceRuntime()
	defer impl.runtime.ReturnCadenceRuntime(runtime)

	value, err := runtime.ReadStored(
		common.MustBytesToAddress(service.Bytes()),
		path)

	const warningMsg = "failed to read contract authorized accounts from " +
		"service account. using default behaviour instead."

	if err != nil {
		impl.logger.Warn().Msg(warningMsg)
		return defaultAccounts
	}
	addresses, ok := cadenceValueToAddressSlice(value)
	if !ok {
		impl.logger.Warn().Msg(warningMsg)
		return defaultAccounts
	}
	return addresses
}

type ContractUpdaterImpl struct {
	tracer          tracing.TracerSpan
	meter           Meter
	accounts        Accounts
	signingAccounts []flow.Address

	draftUpdates map[common.AddressLocation]ContractUpdate

	ContractUpdaterStubs
}

var _ ContractUpdater = &ContractUpdaterImpl{}

func NewContractUpdaterForTesting(
	accounts Accounts,
	stubs ContractUpdaterStubs,
) *ContractUpdaterImpl {
	updater := NewContractUpdater(
		tracing.NewTracerSpan(),
		nil,
		accounts,
		nil,
		nil,
		DefaultContractUpdaterParams(),
		nil,
		nil,
		nil)
	updater.ContractUpdaterStubs = stubs
	return updater
}

func NewContractUpdater(
	tracer tracing.TracerSpan,
	meter Meter,
	accounts Accounts,
	signingAccounts []flow.Address,
	chain flow.Chain,
	params ContractUpdaterParams,
	logger *ProgramLogger,
	systemContracts *SystemContracts,
	runtime *Runtime,
) *ContractUpdaterImpl {
	updater := &ContractUpdaterImpl{
		tracer:          tracer,
		meter:           meter,
		accounts:        accounts,
		signingAccounts: signingAccounts,
		ContractUpdaterStubs: &contractUpdaterStubsImpl{
			logger:                logger,
			chain:                 chain,
			ContractUpdaterParams: params,
			systemContracts:       systemContracts,
			runtime:               runtime,
		},
	}

	updater.Reset()
	return updater
}

func (updater *ContractUpdaterImpl) UpdateAccountContractCode(
	location common.AddressLocation,
	code []byte,
) error {
	defer updater.tracer.StartChildSpan(
		trace.FVMEnvUpdateAccountContractCode).End()

	err := updater.meter.MeterComputation(
		ComputationKindUpdateAccountContractCode,
		1)
	if err != nil {
		return fmt.Errorf("update account contract code failed: %w", err)
	}

	err = updater.SetContract(
		location,
		code,
		updater.signingAccounts)
	if err != nil {
		return fmt.Errorf("updating account contract code failed: %w", err)
	}

	return nil
}

func (updater *ContractUpdaterImpl) RemoveAccountContractCode(
	location common.AddressLocation,
) error {
	defer updater.tracer.StartChildSpan(
		trace.FVMEnvRemoveAccountContractCode).End()

	err := updater.meter.MeterComputation(
		ComputationKindRemoveAccountContractCode,
		1)
	if err != nil {
		return fmt.Errorf("remove account contract code failed: %w", err)
	}

	err = updater.RemoveContract(
		location,
		updater.signingAccounts)
	if err != nil {
		return fmt.Errorf("remove account contract code failed: %w", err)
	}

	return nil
}

func (updater *ContractUpdaterImpl) SetContract(
	location common.AddressLocation,
	code []byte,
	signingAccounts []flow.Address,
) error {
	// Initial contract deployments must be authorized by signing accounts.
	//
	// Contract updates are always allowed.
	exists, err := updater.accounts.ContractExists(location.Name, flow.ConvertAddress(location.Address))
	if err != nil {
		return err
	}

	if !exists && !updater.isAuthorizedForDeployment(signingAccounts) {
		return fmt.Errorf(
			"deploying contract failed: %w",
			errors.NewOperationAuthorizationErrorf(
				"SetContract",
				"deploying contracts requires authorization from specific "+
					"accounts"))

	}

	updater.draftUpdates[location] = ContractUpdate{
		Location: location,
		Code:     code,
	}

	return nil
}

func (updater *ContractUpdaterImpl) RemoveContract(
	location common.AddressLocation,
	signingAccounts []flow.Address,
) (err error) {
	// check if authorized
	if !updater.isAuthorizedForRemoval(signingAccounts) {
		return fmt.Errorf("removing contract failed: %w",
			errors.NewOperationAuthorizationErrorf(
				"RemoveContract",
				"removing contracts requires authorization from specific "+
					"accounts"))
	}

	u := ContractUpdate{Location: location}
	updater.draftUpdates[location] = u

	return nil
}

func (updater *ContractUpdaterImpl) Commit() (ContractUpdates, error) {
	updateList := updater.updates()
	updater.Reset()

	contractUpdates := ContractUpdates{
		Updates:   make([]common.AddressLocation, 0, len(updateList)),
		Deploys:   make([]common.AddressLocation, 0, len(updateList)),
		Deletions: make([]common.AddressLocation, 0, len(updateList)),
	}

	var err error
	for _, v := range updateList {
		var currentlyExists bool
		currentlyExists, err = updater.accounts.ContractExists(v.Location.Name, flow.ConvertAddress(v.Location.Address))
		if err != nil {
			return ContractUpdates{}, err
		}
		shouldDelete := len(v.Code) == 0

		if shouldDelete {
			// this is a removal
			contractUpdates.Deletions = append(contractUpdates.Deletions, v.Location)
			err = updater.accounts.DeleteContract(v.Location.Name, flow.ConvertAddress(v.Location.Address))
			if err != nil {
				return ContractUpdates{}, err
			}
		} else {
			if !currentlyExists {
				// this is a deployment
				contractUpdates.Deploys = append(contractUpdates.Deploys, v.Location)
			} else {
				// this is an update
				contractUpdates.Updates = append(contractUpdates.Updates, v.Location)
			}

			err = updater.accounts.SetContract(v.Location.Name, flow.ConvertAddress(v.Location.Address), v.Code)
			if err != nil {
				return ContractUpdates{}, err
			}
		}
	}

	return contractUpdates, nil
}

func (updater *ContractUpdaterImpl) Reset() {
	updater.draftUpdates = make(map[common.AddressLocation]ContractUpdate)
}

func (updater *ContractUpdaterImpl) HasUpdates() bool {
	return len(updater.draftUpdates) > 0
}

func (updater *ContractUpdaterImpl) updates() []ContractUpdate {
	if len(updater.draftUpdates) == 0 {
		return nil
	}
	keys := make([]common.AddressLocation, 0, len(updater.draftUpdates))
	updates := make([]ContractUpdate, 0, len(updater.draftUpdates))
	for key, update := range updater.draftUpdates {
		keys = append(keys, key)
		updates = append(updates, update)
	}

	sort.Sort(&sortableContractUpdates{keys: keys, updates: updates})
	return updates
}

func (updater *ContractUpdaterImpl) isAuthorizedForDeployment(
	signingAccounts []flow.Address,
) bool {
	if updater.RestrictedDeploymentEnabled() {
		return updater.isAuthorized(
			signingAccounts,
			blueprints.ContractDeploymentAuthorizedAddressesPath)
	}
	return true
}

func (updater *ContractUpdaterImpl) isAuthorizedForRemoval(
	signingAccounts []flow.Address,
) bool {
	if updater.RestrictedRemovalEnabled() {
		return updater.isAuthorized(
			signingAccounts,
			blueprints.ContractRemovalAuthorizedAddressesPath)
	}
	return true
}

func (updater *ContractUpdaterImpl) isAuthorized(
	signingAccounts []flow.Address,
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

func cadenceValueToAddressSlice(value cadence.Value) (
	[]flow.Address,
	bool,
) {
	v, ok := value.(cadence.Array)
	if !ok {
		return nil, false
	}

	addresses := make([]flow.Address, 0, len(v.Values))
	for _, value := range v.Values {
		a, ok := value.(cadence.Address)
		if !ok {
			return nil, false
		}
		addresses = append(addresses, flow.ConvertAddress(a))
	}
	return addresses, true
}
