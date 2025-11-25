package validation

import "fmt"

// ValidateMinMaxSealingLag validates that the minimum sealing lag is not greater than the maximum sealing lag.
func ValidateMinMaxSealingLag(minSealingLag uint, maxSealingLag uint) error {
	if minSealingLag > maxSealingLag {
		return fmt.Errorf("invalid sealing lag parameters: minSealingLag (%v) > maxSealingLag (%v)", minSealingLag, maxSealingLag)
	}
	return nil
}

// ValidateHalvingInterval validates that the halving interval is greater than zero.
func ValidateHalvingInterval(halvingInterval uint) error {
	if halvingInterval == 0 {
		return fmt.Errorf("halving interval must be greater than zero")
	}
	return nil
}
