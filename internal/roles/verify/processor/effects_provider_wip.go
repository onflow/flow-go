package processor

import (
	log "github.com/sirupsen/logrus"

	"github.com/dapperlabs/bamboo-node/internal/roles/verify/compute"
)

// EffectsProvider implements the Effects interface.
// Note: this is still a WIP and blocked on progress of features outside of the verifier role (gossip layer, stakes, etc').
type EffectsProvider struct {
}

func NewEffectsProvider() Effects {
	return &EffectsProvider{}
}

func (e *EffectsProvider) IsValidExecutionReceipt(*ExecutionReceipt) (compute.ValidationResult, error) {
	return &compute.ValidationResultSuccess{}, nil
}

func (e *EffectsProvider) HasMinStake(*ExecutionReceipt) (bool, error) {
	return true, nil
}

func (e *EffectsProvider) IsSealedWithDifferentReceipt(*ExecutionReceipt) (bool, error) {
	return false, nil
}

func (e *EffectsProvider) Send(*ExecutionReceipt, []byte) error {
	return nil
}

func (e *EffectsProvider) SlashExpiredReceipt(*ExecutionReceipt) error {
	return nil
}

func (e *EffectsProvider) SlashInvalidReceipt(*ExecutionReceipt, *compute.BlockPartExecutionResult) error {
	return nil
}

func (e *EffectsProvider) HandleError(err error) {
	log.Errorf("receipt processor errored: %v", err)
}
