package handler

import "fmt"

type ComputationCostLabel string

// ComputationMeter meters computation usage
type ComputationMeter interface {
	// Limit gets computation limit
	Limit() uint64
	// AddUsed adds more computation used to the current computation used
	AddUsed(amount uint64, label ComputationCostLabel) error
	// Used gets the current computation used
	Used() uint64
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
	used    uint64
	limit   uint64
	handler *ComputationMeteringHandler
}

func (c *computationMeter) Limit() uint64 {
	return c.limit
}

func (c *computationMeter) AddUsed(amount uint64, label ComputationCostLabel) error {
	c.handler.weights[string(label)] += amount
	costFactor, ok := c.handler.factors[string(label)]
	if !ok {
		costFactor = 1
	}

	c.used += costFactor * amount
	return nil
}

func (c *computationMeter) Used() uint64 {
	return c.used
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
	factors     map[string]uint64
	weights     map[string]uint64
}

func (c *ComputationMeteringHandler) StartSubMeter(limit uint64) SubComputationMeter {
	m := &subComputationMeter{
		computationMeter: computationMeter{
			limit:   limit,
			handler: c,
		},
		parent: c.computation,
	}

	c.computation = m
	return m
}

type ComputationMeteringOption func(*ComputationMeteringHandler)

var _ ComputationMeter = &ComputationMeteringHandler{}

func NewComputationMeteringHandler(computationLimit uint64, options ...ComputationMeteringOption) *ComputationMeteringHandler {
	h := &ComputationMeteringHandler{
		weights: map[string]uint64{},
	}
	h.computation = &computationMeter{
		limit:   computationLimit,
		handler: h,
	}

	for _, o := range options {
		o(h)
	}
	return h
}

func WithCoumputationWeightFactors(weightFactors map[string]uint64) ComputationMeteringOption {
	return func(handler *ComputationMeteringHandler) {
		handler.factors = weightFactors
	}
}

func (c *ComputationMeteringHandler) Limit() uint64 {
	return c.computation.Limit()
}

func (c *ComputationMeteringHandler) AddUsed(amount uint64, label ComputationCostLabel) error {
	return c.computation.AddUsed(amount, label)
}

func (c *ComputationMeteringHandler) Used() uint64 {
	return c.computation.Used()
}

func (c *ComputationMeteringHandler) Weights() map[string]uint64 {
	return c.weights
}
