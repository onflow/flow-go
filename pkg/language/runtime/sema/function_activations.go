package sema

type FunctionActivation struct {
	ReturnType           Type
	Loops                int
	ValueActivationDepth int
}

func (a FunctionActivation) InLoop() bool {
	return a.Loops > 0
}

type FunctionActivations struct {
	activations []*FunctionActivation
}

func (a *FunctionActivations) EnterFunction(functionType *FunctionType, valueActivationDepth int) {
	a.activations = append(a.activations,
		&FunctionActivation{
			ReturnType:           functionType.ReturnTypeAnnotation.Type,
			ValueActivationDepth: valueActivationDepth,
		},
	)
}

func (a *FunctionActivations) LeaveFunction() {
	lastIndex := len(a.activations) - 1
	a.activations = a.activations[:lastIndex]
}

func (a *FunctionActivations) WithFunction(functionType *FunctionType, valueActivationDepth int, f func()) {
	a.EnterFunction(functionType, valueActivationDepth)
	defer a.LeaveFunction()
	f()
}

func (a *FunctionActivations) Current() *FunctionActivation {
	lastIndex := len(a.activations) - 1
	if lastIndex < 0 {
		return nil
	}
	return a.activations[lastIndex]
}

func (a *FunctionActivations) EnterLoop() {
	a.Current().Loops += 1
}

func (a *FunctionActivations) LeaveLoop() {
	a.Current().Loops -= 1
}

func (a *FunctionActivations) WithLoop(f func()) {
	a.EnterLoop()
	defer a.LeaveLoop()
	f()
}
