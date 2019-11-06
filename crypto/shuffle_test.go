package crypto

import (
	"fmt"
	"testing"
)

func TestFisherYatesShuffle(t *testing.T) {
	listSize := 1000
	subsetSize := 20
	seed := []uint64{1, 2, 3, 4, 5, 6}
	shuffledlist := FisherYatesShuffle(listSize, subsetSize, seed)
	if len(shuffledlist) != subsetSize {
		t.Error(fmt.Sprintf("FisherYates returned a list with a wrong size"))
	}
	// check for repetition
	has := make(map[int]bool)
	for i := range shuffledlist {
		if _, ok := has[shuffledlist[i]]; ok {
			t.Error(fmt.Sprintf("dupplicated item in the results returned by FisherYates"))
		}
		has[shuffledlist[i]] = true
	}
}
