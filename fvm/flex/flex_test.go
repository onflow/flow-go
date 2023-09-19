package flex

import (
	"testing"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/flex/storage"
	"github.com/onflow/flow-go/fvm/storage/testutils"
)

func RunWithTempDB(t testing.TB, f func(*storage.Database)) {
	txnState := testutils.NewSimpleTransaction(nil)
	accounts := environment.NewAccounts(txnState)
	db := storage.NewDatabase(accounts)
	f(db)
}

// func Test(t *testing.T) {

// 	RunWithTempDB(t, func(db *storage.Database) {
// 		handler := NewBaseFlexContractHandler(db)
// 		env := runtime.NewBaseInterpreterEnvironment(runtime.Config{})
// 		flex := NewFlexStandardLibraryValue(nil, handler)

// 		runtime.NewInterpreterRuntime(pool.config)

// 		env.Declare(*flex)

// 		script := []byte(`
// 			import Flex

// 			pub fun main(_ data: [UInt8]): [[UInt8]] {
// 				return FLEX.Address(data)
// 			}
// 		`)

// 		data := []cadence.Value{
// 			cadence.UInt8(192),
// 		}

// 		env.

// 	})
// }
