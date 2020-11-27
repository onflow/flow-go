package state

import (
	"github.com/onflow/flow-go/model/flow"
)

const (
	defaultMaxKeySize         = 16_000     // ~16KB
	defaultMaxValueSize       = 32_000_000 // ~32MB
	defaultMaxInteractionSize = 64_000_000 // ~64MB
	defaultGasLimit           = 100_000    // 100K
)

type State struct {
	ledger                Ledger
	interactionUsed       uint64
	maxKeySizeAllowed     uint64
	maxValueSizeAllowed   uint64
	maxInteractionAllowed uint64
}

func NewState(ledger Ledger, maxKeySizeAllowed, maxValueSizeAllowed, maxInteractionAllowed uint64) *State {
	return &State{
		ledger:                ledger,
		interactionUsed:       uint64(0),
		maxKeySizeAllowed:     maxKeySizeAllowed,
		maxValueSizeAllowed:   maxValueSizeAllowed,
		maxInteractionAllowed: maxInteractionAllowed,
	}
}

func (s State) Set(owner, controller, key string, value flow.RegisterValue) error {
	if err := s.checkSize(owner, controller, key, value); err != nil {
		return err
	}
	err := s.ledger.Set(owner, controller, key, value)
	if err != nil {
		// Return fatal erro
	}
	return nil
}

func (s State) Get(owner, controller, key string) (flow.RegisterValue, error) {
	if err := s.checkSize(owner, controller, key, []byte{}); err != nil {
		return nil, err
	}

	value, err := s.ledger.Get(owner, controller, key)
	if err != nil {
		// Return fatal error
	}
	return value, nil
}

func (s State) Touch(owner, controller, key string) error {
	if err := s.checkSize(owner, controller, key, []byte{}); err != nil {
		return err
	}
	err := s.ledger.Touch(owner, controller, key)
	if err != nil {
		// Return fatal error
	}
	return nil
}

func (s State) checkSize(owner, controller, key string, value flow.RegisterValue) error {
	keySize := uint64(len(owner) + len(controller) + len(key))
	valueSize := uint64(len(value))
	if keySize > s.maxKeySizeAllowed {
		return &StateKeySizeLimitError{Owner: owner,
			Controller: controller,
			Key:        key,
			Size:       keySize,
			Limit:      s.maxKeySizeAllowed}
	}
	if valueSize > s.maxValueSizeAllowed {
		return &StateValueSizeLimitError{Value: value,
			Size:  keySize,
			Limit: s.maxKeySizeAllowed}
	}
	s.interactionUsed += keySize + valueSize
	if s.interactionUsed > s.maxInteractionAllowed {
		return &StateInteractionLimitExceededError{
			Used:  s.interactionUsed,
			Limit: s.maxInteractionAllowed}
	}
	return nil
}

func (s State) InteractionUsed() uint64 {
	return s.interactionUsed
}

func (s State) MaxKeySizeAllowed() uint64 {
	return s.maxKeySizeAllowed
}

func (s State) MaxInteractionAllowed() uint64 {
	return s.maxInteractionAllowed
}

func (s State) MaxValueSizeAllowed() uint64 {
	return s.maxValueSizeAllowed
}
