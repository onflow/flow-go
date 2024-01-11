package main

import "sync"

// TODO: move into controller
var desiredMaxTime float64 = 500 // in milliseconds

type TransactionLengthController struct {
	mu sync.Mutex

	slopePoints uint64
	slope       float64

	maxLoopLength uint64
}

func (c *TransactionLengthController) AdjustParameterRange(parameter uint64, executionTime uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prevParamMax := c.maxLoopLength

	if parameter == 0 {
		return
	}

	c.slope = (float64(c.slopePoints)*c.slope + (float64(executionTime) / float64(parameter))) / float64(c.slopePoints+1)
	c.slopePoints++

	c.maxLoopLength = uint64(desiredMaxTime / c.slope)
	if c.maxLoopLength < 1 {
		c.maxLoopLength = 1
	}
	if (float64(c.maxLoopLength)-float64(prevParamMax))/float64(prevParamMax) > 2.0 {
		c.maxLoopLength = prevParamMax * 2
	}
}

type TransactionLengthControllers map[TemplateName]*TransactionLengthController

func NewTransactionLengthControllers(templates []TransactionTemplate) TransactionLengthControllers {
	controllers := make(TransactionLengthControllers)
	for _, template := range templates {
		switch t := template.(type) {
		case TransactionBodyTemplate:
			controllers[t.Name()] = &TransactionLengthController{
				slopePoints:   1,
				slope:         1,
				maxLoopLength: t.InitialLoopLength(),
			}
		}
	}
	return controllers

}

func (s TransactionLengthControllers) AdjustParameterRange(template TemplateName, parameter uint64, executionTime uint64) {
	controller, ok := s[template]
	if !ok {
		// not all templates have a controller
		return
	}
	controller.AdjustParameterRange(parameter, executionTime)
}

func (s TransactionLengthControllers) GetLoopLength(name TemplateName) uint64 {
	controller, ok := s[name]
	if !ok {
		// not all templates have a controller
		return 1
	}
	return controller.maxLoopLength

}
