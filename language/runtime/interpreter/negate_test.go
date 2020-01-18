package interpreter

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
)

func TestNegate(t *testing.T) {

	t.Run("Int8", func(t *testing.T) {
		assert.Panics(t, func() {
			Int8Value(math.MinInt8).Negate()
		})
	})

	t.Run("Int16", func(t *testing.T) {
		assert.Panics(t, func() {
			Int16Value(math.MinInt16).Negate()
		})
	})

	t.Run("Int32", func(t *testing.T) {
		assert.Panics(t, func() {
			Int32Value(math.MinInt32).Negate()
		})
	})

	t.Run("Int64", func(t *testing.T) {
		assert.Panics(t, func() {
			Int64Value(math.MinInt64).Negate()
		})
	})

	t.Run("Int128", func(t *testing.T) {
		assert.Panics(t, func() {
			Int128Value{big.NewInt(0).Set(sema.Int128TypeMin)}.Negate()
		})
	})
}
