package collection

import (
	"fmt"
	"testing"
)

func TestStepHalving(t *testing.T) {
	min := uint(20)
	max := uint(50)
	for i := 0; i < 100; i++ {
		x := i
		interval := 5
		y := StepHalving([2]uint{min, max}, [2]uint{1, 100}, uint(x), uint(interval))
		fmt.Printf("%d, %d\n", x, y)
	}
}
