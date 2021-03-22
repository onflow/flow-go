package state

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

// A StateKeySizeLimitError indicates that the provided key has exceeded the size limit allowed by the storage
type StateKeySizeLimitError struct {
	Owner      string
	Controller string
	Key        string
	Size       uint64
	Limit      uint64
}

func (e *StateKeySizeLimitError) Error() string {
	return fmt.Sprintf("key %s has size %d which is higher than storage key size limit %d.", fullKey(e.Owner, e.Controller, e.Key), e.Size, e.Limit)
}

// A StateValueSizeLimitError indicates that the provided value has exceeded the size limit allowed by the storage
type StateValueSizeLimitError struct {
	Value flow.RegisterValue
	Size  uint64
	Limit uint64
}

func (e *StateValueSizeLimitError) Error() string {
	return fmt.Sprintf("value %s has size %d which is higher than storage value size limit %d.", string(e.Value[0:10])+"..."+string(e.Value[len(e.Value)-10:]), e.Size, e.Limit)
}

// A StateInteractionLimitExceededError
type StateInteractionLimitExceededError struct {
	Used  uint64
	Limit uint64
}

func (e *StateInteractionLimitExceededError) Error() string {
	return fmt.Sprintf("max interaction with storage has exceeded the limit (used: %d, limit %d)", e.Used, e.Limit)
}

type LedgerFailure struct {
	err error
}

func (e *LedgerFailure) Error() string {
	return fmt.Sprintf("ledger returns unsuccessful: %s", e.err.Error())
}

func fullKey(owner, controller, key string) string {
	// https://en.wikipedia.org/wiki/C0_and_C1_control_codes#Field_separators
	return strings.Join([]string{owner, controller, key}, "\x1F")
}
