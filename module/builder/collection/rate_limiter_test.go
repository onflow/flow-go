package collection

import (
	"fmt"
	"testing"
)

// StepHalving implements a step halving function.
func TestStepHalving(t *testing.T) {
	min := uint(25)
	max := uint(55)
	for i := 0; i < 100; i++ {
		x := i
		interval := max - min
		y := StepHalving([2]uint{min, max}, [2]uint{0, 100}, uint(x), uint(interval))
		fmt.Printf("%d, %d\n", x, y)
	}
}
