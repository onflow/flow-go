package handler

import (
	"errors"
	"sync"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type ContractUpdateKey struct {
	address flow.Address
	name    string
}

type ContractUpdate struct {
	ContractUpdateKey
	Code []byte
}

// ContractHandler handles all interaction
// with smart contracts such as get/set/update
// it also captures all changes as deltas and
// only commit them when called so smart contract
// updates can be delayed until end of the tx execution
type ContractHandler struct {
	accounts                    *state.Accounts
	draftUpdates                map[ContractUpdateKey]ContractUpdate
	restrictedDeploymentEnabled bool
	authorizedAccounts          []runtime.Address
	// handler doesn't have to be thread safe and right now
	// is only used in a single thread but a mutex has been added
	// here to prevent accidental multi-thread use in the future
	lock sync.Mutex
}

func NewContractHandler(accounts *state.Accounts, restrictedDeploymentEnabled bool, authorizedAccounts []runtime.Address) *ContractHandler {
	return &ContractHandler{
		accounts:                    accounts,
		draftUpdates:                make(map[ContractUpdateKey]ContractUpdate),
		restrictedDeploymentEnabled: restrictedDeploymentEnabled,
		authorizedAccounts:          authorizedAccounts,
	}
}

func (h *ContractHandler) GetContractNames(address runtime.Address) (names []string, err error) {
	names, err = h.accounts.GetContractNames(flow.Address(address))
	return
}

func (h *ContractHandler) GetContract(address runtime.Address, name string) (code []byte, err error) {
	code, err = h.accounts.GetContract(name, flow.Address(address))
	return
}

func (h *ContractHandler) SetContract(address runtime.Address, name string, code []byte, signingAccounts []runtime.Address) (err error) {
	// check if authorized
	if !h.isAuthorized(signingAccounts) {
		return errors.New("code deployment requires authorization from specific accounts")
	}
	add := flow.Address(address)
	h.lock.Lock()
	defer h.lock.Unlock()
	uk := ContractUpdateKey{add, name}
	u := ContractUpdate{uk, code}
	h.draftUpdates[uk] = u

	return nil
}

func (h *ContractHandler) RemoveContract(address runtime.Address, name string, signingAccounts []runtime.Address) (err error) {
	// check if authorized
	if !h.isAuthorized(signingAccounts) {
		return errors.New("code deployment requires authorization from specific accounts")
	}

	add := flow.Address(address)
	// removes are stored in the draft updates with code value of nil
	h.lock.Lock()
	defer h.lock.Unlock()
	uk := ContractUpdateKey{add, name}
	u := ContractUpdate{uk, nil}
	h.draftUpdates[uk] = u

	return nil
}

func (h *ContractHandler) Commit() error {

	h.lock.Lock()
	defer h.lock.Unlock()

	var err error
	for _, v := range h.draftUpdates {
		if len(v.Code) > 0 {
			err = h.accounts.SetContract(v.name, v.address, v.Code)
			if err != nil {
				return err
			}
		} else {
			err = h.accounts.DeleteContract(v.name, v.address)
			if err != nil {
				return err
			}
		}
	}

	// reset draft
	h.draftUpdates = make(map[ContractUpdateKey]ContractUpdate)
	return nil
}

func (h *ContractHandler) Rollback() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.draftUpdates = make(map[ContractUpdateKey]ContractUpdate)
	return nil
}

func (h *ContractHandler) HasUpdates() bool {
	return len(h.draftUpdates) > 0
}

func (h *ContractHandler) UpdateKeys() []ContractUpdateKey {
	if len(h.draftUpdates) == 0 {
		return nil
	}
	keys := make([]ContractUpdateKey, 0, len(h.draftUpdates))
	for k := range h.draftUpdates {
		keys = append(keys, k)
	}
	return keys
}

func (h *ContractHandler) isAuthorized(signingAccounts []runtime.Address) bool {
	if h.restrictedDeploymentEnabled {
		for _, authorized := range h.authorizedAccounts {
			for _, signer := range signingAccounts {
				if signer == authorized {
					// a single authorized singer is enough
					return true
				}
			}
		}
		return false
	}
	return true
}
