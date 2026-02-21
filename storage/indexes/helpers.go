package indexes

import (
	"fmt"
	"math"
)

func validateLimit(limit uint32) error {
	if limit == 0 {
		return fmt.Errorf("limit must be greater than 0")
	}
	if limit == math.MaxUint32 {
		return fmt.Errorf("limit must be less than %d", math.MaxUint32)
	}
	return nil
}
