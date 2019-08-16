package oldcrypto

import (
	"fmt"
)

// InvalidSeedError indicates that a seed could not be used to generate a master extended key.
type InvalidSeedError struct {
	seed string
}

func (e *InvalidSeedError) Error() string {
	return fmt.Sprintf("Invalid seed: %s", e.seed)
}
