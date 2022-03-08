package handler

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
)

type MetringOperationType uint

const (
	// 10_000 to 11_000 for FVM
	_ = iota + 10_000
	MeteredOperationHash
	MeteredOperationfunction_or_loop_call
	MeteredOperationVerifySignature
	MeteredOperationContractFunctionInvoke
	MeteredOperationAddAccountKey
	MeteredOperationAddEncodedAccountKey
	MeteredOperationAllocateStorageIndex
	MeteredOperationCreateAccount
	MeteredOperationEmitEvent
	MeteredOperationGenerateUUID
	MeteredOperationGetAccountAvailableBalance
	MeteredOperationGetAccountBalance
	MeteredOperationGetAccountContractCode
	MeteredOperationGetAccountContractNames
	MeteredOperationGetAccountKey
	MeteredOperationGetBlockAtHeight
	MeteredOperationGetCode
	MeteredOperationGetCurrentBlockHeight
	MeteredOperationGetProgram
	MeteredOperationGetStorageCapacity
	MeteredOperationGetStorageUsed
	MeteredOperationGetValue
	MeteredOperationRemoveAccountContractCode
	MeteredOperationResolveLocation
	MeteredOperationRevokeAccountKey
	MeteredOperationRevokeEncodedAccountKey
	MeteredOperationSetProgram
	MeteredOperationSetValue
	MeteredOperationUpdateAccountContractCode
	MeteredOperationValidatePublicKey
	MeteredOperationValueExists
)

// ComputationMeter meters computation usage
type ComputationMeter interface {
	// Limit gets computation limit
	Limit() uint64
	// AddUsed adds more computation used to the current computation using the type specified
	AddUsed(feature uint, intensity uint) error
	// Used gets the current computation used
	Used() uint64
	// Intensities are the aggregated intensities of all the feature calls so far. Used for logging.
	Intensities() map[uint]uint
}

// SubComputationMeter meters computation usage. Currently, can only be discarded,
// which can be used to meter fees separately from the transaction invocation.
// A future expansion is to meter (and charge?) different part of the transaction separately
type SubComputationMeter interface {
	ComputationMeter
	// Discard discards this computation metering.
	// This resets the ComputationMeteringHandler to the previous ComputationMeter,
	// without updating the limit or computation used
	Discard() error
	// TODO: this functionality isn't currently needed for anything.
	// It might be needed in the future, if we need to meter any execution separately and then include it in the base metering.
	// I left this TODO just as a reminder that this was the idea.
	// Commit()
}

type computationMeter struct {
	used        uint64
	limit       uint64
	intensities map[uint]uint
	handler     *ComputationMeteringHandler
}

func (c *computationMeter) Limit() uint64 {
	return c.limit
}

func (c *computationMeter) AddUsed(feature uint, intensity uint) error {
	// take note of the intensity even if the weight of the feature is 0
	c.intensities[feature] += intensity
	weight, ok := c.handler.weights[feature]
	if !ok {
		return nil
	}

	c.used += uint64(weight * intensity)
	return c.errorIfOverLimit()
}

func (c *computationMeter) errorIfOverLimit() error {
	if c.used > c.limit {
		return runtime.ComputationLimitExceededError{
			Limit: c.limit,
		}
	}
	return nil
}

func (c *computationMeter) Used() uint64 {
	return c.used
}

func (c *computationMeter) Intensities() map[uint]uint {
	return c.intensities
}

var _ ComputationMeter = &computationMeter{}

type subComputationMeter struct {
	computationMeter
	parent ComputationMeter
}

func (s *subComputationMeter) Discard() error {
	if s.handler.computation != s {
		return fmt.Errorf("cannot discard non-outermost SubComputationMeter")
	}
	s.handler.computation = s.parent
	return nil
}

var _ SubComputationMeter = &subComputationMeter{}

type ComputationMeteringHandler struct {
	computation ComputationMeter
	weights     map[uint]uint
}

func (c *ComputationMeteringHandler) StartSubMeter(limit uint64) SubComputationMeter {
	m := &subComputationMeter{
		computationMeter: computationMeter{
			limit:       limit,
			intensities: map[uint]uint{},
			handler:     c,
		},
		parent: c.computation,
	}

	c.computation = m
	return m
}

type ComputationMeteringOption func(*ComputationMeteringHandler)

var _ ComputationMeter = &ComputationMeteringHandler{}

func ComputationLimit(ctxGasLimit, txGasLimit, defaultGasLimit uint64) uint64 {
	// if gas limit is set to zero fallback to the gas limit set by the context
	if txGasLimit == 0 {
		// if context gasLimit is also zero, fallback to the default gas limit
		if ctxGasLimit == 0 {
			return defaultGasLimit
		}
		return ctxGasLimit
	}
	return txGasLimit
}

func NewComputationMeteringHandler(computationLimit uint64, options ...ComputationMeteringOption) *ComputationMeteringHandler {
	h := &ComputationMeteringHandler{}
	h.computation = &computationMeter{
		limit:       computationLimit,
		intensities: map[uint]uint{},
		handler:     h,
	}

	for _, o := range options {
		o(h)
	}
	return h
}

func WithCoumputationWeightFactors(weights map[uint]uint) ComputationMeteringOption {
	return func(handler *ComputationMeteringHandler) {
		handler.weights = weights
	}
}

func (c *ComputationMeteringHandler) Limit() uint64 {
	return c.computation.Limit()
}

func (c *ComputationMeteringHandler) AddUsed(feature uint, intensity uint) error {
	return c.computation.AddUsed(feature, intensity)
}

func (c *ComputationMeteringHandler) Used() uint64 {
	return c.computation.Used()
}

func (c *ComputationMeteringHandler) Intensities() map[uint]uint {
	return c.computation.Intensities()
}
