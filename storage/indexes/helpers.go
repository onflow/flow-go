package indexes

import (
	"fmt"
	"math"
)

// validateLimit validates the limit parameter for the index is within the valid exclusive range (0, math.MaxUint32)
//
// Any error indicates the limit is invalid.
func validateLimit(limit uint32) error {
	if limit == 0 {
		return fmt.Errorf("limit must be greater than 0")
	}
	if limit == math.MaxUint32 {
		return fmt.Errorf("limit must be less than %d", math.MaxUint32)
	}
	return nil
}
