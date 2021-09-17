package migrations

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"os"
	"path"
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// BalanceReporter iterates through registers getting the location and balance of all FlowVaults
type BalanceReporter struct {
	Log         zerolog.Logger
	OutputDir   string
	totalSupply uint64
}

func (r *BalanceReporter) filename() string {
	return path.Join(r.OutputDir, fmt.Sprintf("balance_report_%d.json", int32(time.Now().Unix())))
}

type balanceDataPoint struct {
	// Path is the storage path of the composite the vault was found in
	Path string `json:"path"`
	// Address is the owner of the composite the vault was found in
	Address string `json:"address"`
	// LastComposite is the Composite directly containing the FlowVault
	LastComposite string `json:"last_composite"`
	// FirstComposite is the root composite at this path which directly or indirectly contains the vault
	FirstComposite string `json:"first_composite"`
	// Balance is the balance of the flow vault
	Balance uint64 `json:"balance"`
}

// Report creates a balance_report_*.json file that contains data on all FlowVaults in the state commitment.
// I recommend using gojq to browse through the data, because of the large uint64 numbers which jq won't be able to handle.
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
	resultsWG := &sync.WaitGroup{}
	jobs := make(chan ledger.Payload)
	resultsChan := make(chan balanceDataPoint, 100)

	workerCount := runtime.NumCPU()

	results := make([]balanceDataPoint, 0)

	resultsWG.Add(1)
	go func() {
		for point := range resultsChan {
			results = append(results, point)
		}
		resultsWG.Done()
	}()

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go r.balanceReporterWorker(jobs, wg, resultsChan)
	}

	wg.Add(1)
	go func() {
		for _, p := range payload {
			jobs <- p
		}

		close(jobs)
		wg.Done()
	}()

	wg.Wait()

	//drain results chan
	close(resultsChan)
	resultsWG.Wait()

	tc, err := json.Marshal(struct {
		Data        []balanceDataPoint
		TotalSupply uint64
	}{
		Data:        results,
		TotalSupply: r.totalSupply,
	})
	if err != nil {
		panic(err)
	}
	_, err = writer.Write(tc)
	if err != nil {
		panic(err)
	}

	return nil
}

func (r *BalanceReporter) balanceReporterWorker(jobs chan ledger.Payload, wg *sync.WaitGroup, dataChan chan balanceDataPoint) {
	for payload := range jobs {
		err := r.handlePayload(payload, dataChan)
		if err != nil {
			r.Log.Err(err).Msg("Error handling payload")
		}
	}

	wg.Done()
}

func (r *BalanceReporter) handlePayload(p ledger.Payload, dataChan chan balanceDataPoint) error {
	id, err := keyToRegisterID(p.Key)
	if err != nil {
		return err
	}

	// Ignore known payload keys that are not Cadence values
	if state.IsFVMStateKey(id.Owner, id.Controller, id.Key) {
		return nil
	}

	value, version := interpreter.StripMagic(p.Value)

	err = storageMigrationV5DecMode.Valid(value)
	if err != nil {
		return nil
	}

	decodeFunction := interpreter.DecodeValue
	if version <= 4 {
		decodeFunction = interpreter.DecodeValueV4
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

	if id.Key == "contract\u001fFlowToken" {
		tokenSupply := uint64(cValue.(*interpreter.CompositeValue).GetField("totalSupply").(interpreter.UFix64Value))
		r.Log.Info().Uint64("tokenSupply", tokenSupply).Msg("total token supply")
		r.totalSupply = tokenSupply
	}

	lastComposite := "none"
	firstComposite := ""

	balanceVisitor := &interpreter.EmptyVisitor{
		CompositeValueVisitor: func(inter *interpreter.Interpreter, value *interpreter.CompositeValue) bool {
			if firstComposite == "" {
				firstComposite = string(value.TypeID())
			}

			if string(value.TypeID()) == "A.1654653399040a61.FlowToken.Vault" {
				b := uint64(value.GetField("balance").(interpreter.UFix64Value))
				if b == 0 {
					// ignore 0 balance results
					return false
				}

				dataChan <- balanceDataPoint{
					Path:           id.Key,
					Address:        flow.BytesToAddress([]byte(id.Owner)).Hex(),
					LastComposite:  lastComposite,
					FirstComposite: firstComposite,
					Balance:        b,
				}

				return false
			}
			lastComposite = string(value.TypeID())
			return true
		},
		DictionaryValueVisitor: func(interpreter *interpreter.Interpreter, value *interpreter.DictionaryValue) bool {
			return value.DeferredKeys() == nil
		},
	}

	inter, err := interpreter.NewInterpreter(nil, common.StringLocation("somewhere"))
	if err != nil {
		return err
	}
	cValue.Accept(inter, balanceVisitor)

	return nil
}
