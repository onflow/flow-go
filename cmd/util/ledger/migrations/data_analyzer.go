package migrations

import (
	"encoding/hex"
	"fmt"
	"math"
	"runtime"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"
	"github.com/schollz/progressbar/v3"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
)

type DataAnalyzer struct {
	Log          zerolog.Logger
	AnalyzeValue func(value interpreter.Value) (bool, error)
}

type dataAnalyzerResult struct {
	key string
	err error
}

func (a DataAnalyzer) Report(payloads []ledger.Payload) error {

	jobs := make(chan ledger.Payload)
	results := make(chan dataAnalyzerResult)

	workerCount := runtime.NumCPU()

	for i := 0; i < workerCount; i++ {
		go a.analyzePayloads(jobs, results)
	}

	go func() {
		for _, payload := range payloads {
			jobs <- payload
		}

		close(jobs)
	}()

	processed := 0
	totalCount := len(payloads)

	bar := progressbar.Default(int64(totalCount))

	for result := range results {
		processed++

		err := bar.Set(processed)
		if err != nil {
			return err
		}

		if result.err != nil {
			return fmt.Errorf("failed to analyze key: %#+v: %w", result.key, result.err)
		}

		if processed == totalCount {
			break
		}
	}

	return nil
}

func (a DataAnalyzer) analyzePayloads(jobs <-chan ledger.Payload, results chan<- dataAnalyzerResult) {
	for payload := range jobs {
		err := a.analyzePayload(payload)
		results <- dataAnalyzerResult{
			key: payload.Key.String(),
			err: err,
		}
	}
}

var dataAnalyzerDecMode = func() cbor.DecMode {
	decMode, err := cbor.DecOptions{
		IntDec:           cbor.IntDecConvertNone,
		MaxArrayElements: math.MaxInt32,
		MaxMapPairs:      math.MaxInt32,
		MaxNestedLevels:  256,
	}.DecMode()
	if err != nil {
		panic(err)
	}
	return decMode
}()

func (r DataAnalyzer) analyzePayload(payload ledger.Payload) error {

	keyParts := payload.Key.KeyParts

	rawOwner := keyParts[0].Value
	rawController := keyParts[1].Value
	rawKey := keyParts[2].Value

	// Ignore known payload keys that are not Cadence values

	if state.IsFVMStateKey(string(rawOwner), string(rawController), string(rawKey)) {
		return nil
	}

	data, version := interpreter.StripMagic(payload.Value)

	err := dataAnalyzerDecMode.Valid(data)
	if err != nil {
		return nil
	}

	// Extract the owner from the key and re-encode the value

	owner := common.BytesToAddress(rawOwner)

	// Decode the value

	key := string(rawKey)
	path := []string{key}

	value, err := interpreter.DecodeValue(data, &owner, path, version, nil)
	if err != nil {

		// Ignore decoding error caused by unsupported storage reference

		// TODO: improve: use dedicated error introduced in https://github.com/onflow/cadence/pull/1064 once available
		message := err.Error()
		if strings.Contains(message, "unsupported decoded tag") &&
			strings.HasSuffix(message, ": 202") {

			return nil
		}

		return fmt.Errorf(
			"failed to decode value: %w\n\nvalue:\n%s\n",
			err,
			hex.Dump(data),
		)
	}

	interpreter.InspectValue(value, func(value interpreter.Value) bool {

		if err != nil {
			return false
		}

		var resume bool
		resume, err = r.AnalyzeValue(value)
		return !resume || err != nil
	})

	return err
}
