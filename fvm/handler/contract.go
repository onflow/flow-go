package handler

import (
	"errors"
	"strings"
	"sync"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type ContractHandler struct {
	accounts                    *state.Accounts
	draftUpdates                map[string]contractUpdate
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
		draftUpdates:                make(map[string]contractUpdate),
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
	u := contractUpdate{add, name, code}
	h.draftUpdates[u.key()] = u

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
	u := contractUpdate{add, name, nil}
	h.draftUpdates[u.key()] = u

	return nil
}

func (h *ContractHandler) Commit() error {

	h.lock.Lock()
	defer h.lock.Unlock()

	var err error
	for _, v := range h.draftUpdates {
		if len(v.code) > 0 {
			err = h.accounts.SetContract(v.name, v.address, v.code)
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
	h.draftUpdates = make(map[string]contractUpdate)
	return nil
}

func (h *ContractHandler) Rollback() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.draftUpdates = make(map[string]contractUpdate)
	return nil
}

func (h *ContractHandler) HasUpdates() bool {
	return len(h.draftUpdates) > 0
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

type contractUpdate struct {
	address flow.Address
	name    string
	code    []byte
}

func (c *contractUpdate) key() string {
	return strings.Join([]string{c.address.String(), c.name}, "\x1F")
}
