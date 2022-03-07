package handler

import "fmt"

// ComputationMeter meters computation usage
type ComputationMeter interface {
	// Limit gets computation limit
	Limit() uint64
	// AddUsed adds more computation used to the current computation used
	AddUsed(intensity uint64, feature string)
	// Used gets the current computation used
	Used() uint64
	// Intensities gets the intensities of features
	Intensities() map[string]uint64
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
	intensities map[string]uint64
	handler     *ComputationMeteringHandler
}

func (c *computationMeter) Limit() uint64 {
	return c.limit
}

func (c *computationMeter) AddUsed(intensity uint64, feature string) {
	c.intensities[feature] += intensity
	if feature != "function_or_loop_call" {
		return
	}
	c.used += intensity
}

func (c *computationMeter) Used() uint64 {
	return c.used
}

func (c *computationMeter) Intensities() map[string]uint64 {
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
}

func (c *ComputationMeteringHandler) StartSubMeter(limit uint64) SubComputationMeter {
	m := &subComputationMeter{
		computationMeter: computationMeter{
			limit:   limit,
			handler: c,
			// sum-meters have separate intensities
			intensities: map[string]uint64{},
		},
		parent: c.computation,
	}

	c.computation = m
	return m
}

func NewComputationMeteringHandler(computationLimit uint64) *ComputationMeteringHandler {
	h := &ComputationMeteringHandler{}
	h.computation = &computationMeter{
		limit:       computationLimit,
		handler:     h,
		intensities: map[string]uint64{},
	}

	return h
}

func (c *ComputationMeteringHandler) Limit() uint64 {
	return c.computation.Limit()
}

func (c *ComputationMeteringHandler) AddUsed(intensity uint64, feature string) {
	c.computation.AddUsed(intensity, feature)
}

func (c *ComputationMeteringHandler) Used() uint64 {
	return c.computation.Used()
}

func (c *ComputationMeteringHandler) Intensities() map[string]uint64 {
	return c.computation.Intensities()
}
