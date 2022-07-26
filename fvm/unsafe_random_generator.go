package fvm

import (
	"encoding/binary"
	"math/rand"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/module/trace"
)

type UnsafeRandomGenerator struct {
	ctx *EnvContext
	rng *rand.Rand
}

func NewUnsafeRandomGenerator(ctx *EnvContext) *UnsafeRandomGenerator {
	gen := &UnsafeRandomGenerator{
		ctx: ctx,
	}

	if ctx.BlockHeader != nil {
		// Seed the random number generator with entropy created from the block
		// header ID. The random number generator will be used by the
		// UnsafeRandom function.
		id := ctx.BlockHeader.ID()
		source := rand.NewSource(int64(binary.BigEndian.Uint64(id[:])))
		gen.rng = rand.New(source)
	}

	return gen
}

// UnsafeRandom returns a random uint64, where the process of random number
// derivation is not cryptographically secure.
func (gen *UnsafeRandomGenerator) UnsafeRandom() (uint64, error) {
	defer gen.ctx.StartExtensiveTracingSpanFromRoot(trace.FVMEnvUnsafeRandom).End()

	if gen.rng == nil {
		return 0, errors.NewOperationNotSupportedError("UnsafeRandom")
	}

	// TODO (ramtin) return errors this assumption that this always succeeds
	// might not be true
	buf := make([]byte, 8)
	_, _ = gen.rng.Read(buf) // Always succeeds, no need to check error
	return binary.LittleEndian.Uint64(buf), nil
}
