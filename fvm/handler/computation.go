package handler

import "fmt"

// ComputationMeter meters computation usage
type ComputationMeter interface {
	// Limit gets computation limit
	Limit() uint64
	// AddUsed adds more computation used to the current computation used
	AddUsed(used uint64) error
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
	// TODO: when needed
	// Commit()
}

// ComputationMeteringHandler handles computation metering on a transaction level
type ComputationMeteringHandler interface {
	ComputationMeter
	StartSubMeter(limit uint64) SubComputationMeter
}

type computationMeter struct {
	used  uint64
	limit uint64
}

func (c *computationMeter) Limit() uint64 {
	return c.limit
}

func (c *computationMeter) AddUsed(used uint64) error {
	c.used += used
	return nil
}

func (c *computationMeter) Used() uint64 {
	return c.used
}

var _ ComputationMeter = &computationMeter{}

type subComputationMeter struct {
	computationMeter
	parent  ComputationMeter
	handler *computationMeteringHandler
}

func (s *subComputationMeter) Discard() error {
	if s.handler.computation != s {
		return fmt.Errorf("cannot discard non-outermost SubComputationMeter")
	}
	s.handler.computation = s.parent
	return nil
}

var _ SubComputationMeter = &subComputationMeter{}

type computationMeteringHandler struct {
	computation ComputationMeter
}

func (c *computationMeteringHandler) StartSubMeter(limit uint64) SubComputationMeter {
	m := &subComputationMeter{
		computationMeter: computationMeter{
			limit: limit,
		},
		parent:  c.computation,
		handler: c,
	}

	c.computation = m
	return m
}

var _ ComputationMeteringHandler = &computationMeteringHandler{}

func NewComputationMeteringHandler(computationLimit uint64) ComputationMeteringHandler {
	return &computationMeteringHandler{
		computation: &computationMeter{
			limit: computationLimit,
		},
	}
}

func (c *computationMeteringHandler) Limit() uint64 {
	return c.computation.Limit()
}

func (c *computationMeteringHandler) AddUsed(used uint64) error {
	used = c.computation.Used() + used
	return c.computation.AddUsed(used)
}

func (c *computationMeteringHandler) Used() uint64 {
	return c.computation.Used()
}
