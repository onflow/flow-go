package ledger_test

import (
	"testing"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
)

func BenchmarkKKK1(b *testing.B) {

	kp1 := utils.KeyPartFixture(1, "key part 1")
	kp2 := utils.KeyPartFixture(22, "key part 2")
	kp3 := utils.KeyPartFixture(25, "key part 3")
	k := ledger.NewKey([]ledger.KeyPart{kp1, kp2, kp3})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = k.CanonicalForm()
	}
	b.StopTimer()
}

/*func BenchmarkKKK2(b *testing.B) {

	kp1 := utils.KeyPartFixture(1, "key part 1")
	kp2 := utils.KeyPartFixture(22, "key part 2")
	kp3 := utils.KeyPartFixture(25, "key part 3")
	k := ledger.NewKey([]ledger.KeyPart{kp1, kp2, kp3})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = k.CanonicalForm()
	}
	b.StopTimer()
}*/
