package migrations

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/flow-go/fvm/state"
	"os"
	"path"
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
)

// iterates through registers keeping a map of register sizes
// reports on storage metrics
type BalanceReporter struct {
	Log       zerolog.Logger
	OutputDir string
}

func (r *BalanceReporter) filename() string {
	return path.Join(r.OutputDir, fmt.Sprintf("balance_report_%d.csv", int32(time.Now().Unix())))
}

type balanceDataBatch struct {
	PathBalance map[string]uint64
}

func newBalanceDataBatch() *balanceDataBatch {
	return &balanceDataBatch{
		PathBalance: make(map[string]uint64),
	}
}

func (d *balanceDataBatch) Join(data *balanceDataBatch) {
	for s, u := range data.PathBalance {
		d.PathBalance[s] = d.PathBalance[s] + u
	}
}

func (r *BalanceReporter) Report(payload []ledger.Payload) error {
	fn := r.filename()
	r.Log.Info().Msgf("Running FLOW balance Reporter. Saving output to %s.", fn)

	f, err := os.Create(fn)
	if err != nil {
		return err
	}

	defer func() {
		err = f.Close()
		if err != nil {
			panic(err)
		}
	}()

	writer := bufio.NewWriter(f)
	defer func() {
		err = writer.Flush()
		if err != nil {
			panic(err)
		}
	}()

	wg := &sync.WaitGroup{}
	jobs := make(chan ledger.Payload)

	workerCount := runtime.NumCPU()

	data := make([]*balanceDataBatch, workerCount)

	for i := 0; i < workerCount; i++ {
		data[i] = newBalanceDataBatch()
		go r.balanceReporterWorker(jobs, wg, data[i])
	}

	go func() {
		wg.Add(1)
		for _, p := range payload {
			jobs <- p
		}

		close(jobs)
		wg.Done()
	}()

	wg.Wait()

	totalData := newBalanceDataBatch()

	for i := 0; i < workerCount; i++ {
		totalData.Join(data[i])
	}

	tc, err := json.Marshal(totalData)
	if err != nil {
		panic(err)
	}
	_, err = writer.Write(tc)
	if err != nil {
		panic(err)
	}

	return nil
}

func (r *BalanceReporter) balanceReporterWorker(jobs chan ledger.Payload, wg *sync.WaitGroup, data *balanceDataBatch) {
	wg.Add(1)

	for payload := range jobs {
		err := r.HandlePayload(payload, data)
		if err != nil {
			r.Log.Err(err).Msg("Error handling payload")
		}
	}

	wg.Done()
}

func (r *BalanceReporter) HandlePayload(p ledger.Payload, data *balanceDataBatch) error {
	id, err := keyToRegisterID(p.Key)
	if err != nil {
		return err
	}

	// Ignore known payload keys that are not Cadence values
	if state.IsFVMStateKey(id.Owner, id.Controller, id.Key) {
		return nil
	}

	value, version := interpreter.StripMagic(p.Value)

	err = decMode.Valid(value)
	if err != nil {
		return nil
	}

	// Determine the appropriate decoder from the decoded version

	decodeFunction := interpreter.DecodeValue
	if version <= 3 {
		decodeFunction = interpreter.DecodeValueV3
	}

	// Decode the value
	owner := common.BytesToAddress([]byte(id.Owner))
	cPath := []string{id.Key}

	cValue, err := decodeFunction(value, &owner, cPath, version, nil)
	if err != nil {
		return fmt.Errorf(
			"failed to decode value: %w\n\nvalue:\n%s\n",
			err, hex.Dump(value),
		)
	}

	interpreterValue, ok := cValue.(interpreter.Value)
	if !ok {
		return nil
	}

	firstCompositeName := ""

	balanceVisitor := &interpreter.EmptyVisitor{
		CompositeValueVisitor: func(inter *interpreter.Interpreter, value *interpreter.CompositeValue) bool {
			if firstCompositeName == "" {
				firstCompositeName = string(value.TypeID())
			}

			if string(value.TypeID()) == "A.1654653399040a61.FlowToken.Vault" {
				b := uint64(value.GetField("balance").(interpreter.UFix64Value))
				data.PathBalance[id.Key] += b
				return false
			}
			return true
		},
		DictionaryValueVisitor: func(interpreter *interpreter.Interpreter, value *interpreter.DictionaryValue) bool {
			return value.DeferredKeys() == nil
		},
	}

	inter, err := interpreter.NewInterpreter(nil, common.StringLocation("somewhere"))
	interpreterValue.Accept(inter, balanceVisitor)

	return nil
}
