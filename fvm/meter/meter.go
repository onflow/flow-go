package meter

type MeterParameters struct {
	ComputationMeterParameters
	MemoryMeterParameters
	EventMeterParameters
	InteractionMeterParameters
}

func DefaultParameters() MeterParameters {
	return MeterParameters{
		ComputationMeterParameters: DefaultComputationMeterParameters(),
		MemoryMeterParameters:      DefaultMemoryParameters(),
		EventMeterParameters:       DefaultEventMeterParameters(),
		InteractionMeterParameters: DefaultInteractionMeterParameters(),
	}
}

// Meter collects memory and computation usage and enforces limits
// for any each memory/computation usage call it sums intensity multiplied by the weight of the intensity to the total
// memory/computation usage metrics and returns error if limits are not met.
type Meter struct {
	MeterParameters

	MemoryMeter
	ComputationMeter
	EventMeter
	InteractionMeter
}

// NewMeter constructs a new Meter
func NewMeter(params MeterParameters) *Meter {
	return &Meter{
		MeterParameters:  params,
		ComputationMeter: NewComputationMeter(params.ComputationMeterParameters),
		MemoryMeter:      NewMemoryMeter(params.MemoryMeterParameters),
		EventMeter:       NewEventMeter(params.EventMeterParameters),
		InteractionMeter: NewInteractionMeter(params.InteractionMeterParameters),
	}
}

// MergeMeter merges the input meter into the current meter
func (m *Meter) MergeMeter(child *Meter) {
	if child == nil {
		return
	}

	m.ComputationMeter.Merge(child.ComputationMeter)
	m.MemoryMeter.Merge(child.MemoryMeter)
	m.EventMeter.Merge(child.EventMeter)
	m.InteractionMeter.Merge(child.InteractionMeter)
}
