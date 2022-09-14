package debug_test

import (
	"fmt"

	"math"
	"testing"

	cadenceRuntime "github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	metering "github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/debug"
)

func TestDebugger_RunTransaction(t *testing.T) {

	grpcAddress := "X.X.X.X:9000"
	address, _ := common.HexToAddress("0xe467b9dd11fa00df")
	blockID, _ := flow.HexStringToIdentifier("88563f69d7394f2658871f09d467d8c14af4023369932f0f10b403be4053f6ab")

	view := debug.NewRemoteView(grpcAddress, debug.WithBlockID(blockID))
	meter := metering.NewMeter(math.MaxUint64, math.MaxUint64)
	st := state.NewState(view, meter)
	sth := state.NewStateHolder(st)
	accounts := state.NewAccounts(sth)
	storage := cadenceRuntime.NewStorage(
		&migrations.AccountsAtreeLedger{Accounts: accounts},
		nil,
	)
	sm := storage.GetStorageMap(address, common.PathDomainStorage.Identifier(), false)

	iter := sm.Iterator(nil)

	fmt.Println("key", "staticType")
	for key, value := iter.Next(); key != ""; key, value = iter.Next() {
		fmt.Println(key, safelyGetStaticType(value))
	}
}

func safelyGetStaticType(value interpreter.Value) (st string) {
	defer func() {
		if r := recover(); r != nil {
			st = "unknown"
		}
	}()
	st = value.StaticType(nil).String()
	return
}
