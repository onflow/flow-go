package migrations

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"runtime"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/ledger"
)

func StorageFormatV4Migration(payloads []ledger.Payload) ([]ledger.Payload, error) {

	migratedPayloads := make([]ledger.Payload, 0, len(payloads))

	jobs := make(chan ledger.Payload)
	results := make(chan struct {
		key string
		ledger.Payload
		error
	})

	workerCount := runtime.NumCPU()

	for i := 0; i < workerCount; i++ {
		go storageFormatV4MigrationWorker(jobs, results)
	}

	go func() {
		for _, payload := range payloads {
			jobs <- payload
		}

		close(jobs)
	}()

	for result := range results {
		if result.error != nil {
			return nil, fmt.Errorf("failed to migrate key: %#+v: %w", result.key, result.error)
		}
		migratedPayloads = append(migratedPayloads, result.Payload)
		if len(migratedPayloads) == len(payloads) {
			break
		}
	}

	return migratedPayloads, nil
}

func storageFormatV4MigrationWorker(jobs <-chan ledger.Payload, results chan<- struct {
	key string
	ledger.Payload
	error
}) {
	for payload := range jobs {
		migratedPayload, err := rencodePayloadV4(payload)
		result := struct {
			key string
			ledger.Payload
			error
		}{
			key: payload.Key.String(),
		}
		if err != nil {
			result.error = err
		} else {
			result.Payload = migratedPayload
		}
		results <- result
	}
}

var publicKeyKeyPrefix = []byte("public_key_")
var codeKeyPrefix = []byte("code.")
var contractNamesKey = []byte("contract_names")
var accountAddressStateKey = []byte("account_address_state")

func rencodePayloadV4(payload ledger.Payload) (ledger.Payload, error) {

	// If the payload value is not a Cadence value (it does not have the Cadence data magic prefix),
	// return the payload as is

	rawOwner := payload.Key.KeyParts[0].Value
	rawController := payload.Key.KeyParts[1].Value
	rawKey := payload.Key.KeyParts[2].Value

	if len(rawOwner) == 0 ||
		!bytes.Equal(rawOwner, rawController) ||
		bytes.HasPrefix(rawKey, publicKeyKeyPrefix) ||
		bytes.HasPrefix(rawKey, codeKeyPrefix) ||
		bytes.Equal(rawKey, contractNamesKey) ||
		bytes.Equal(rawKey, accountAddressStateKey) {

		return payload, nil
	}

	value, version := interpreter.StripMagic(payload.Value)
	if version == 0 {
		return ledger.Payload{}, fmt.Errorf("Cadence payload key has no magic prefix: %s", payload.Key)
	}

	// Extract the owner from the key and re-encode the value

	owner := common.BytesToAddress(payload.Key.KeyParts[0].Value)

	var err error
	payload.Value, err = rencodeValueV4(value, owner, version)
	if err != nil {
		return ledger.Payload{}, err
	}

	payload.Value = interpreter.PrependMagic(
		payload.Value,
		interpreter.CurrentEncodingVersion,
	)

	return payload, nil
}

func rencodeValueV4(data []byte, owner common.Address, version uint16) ([]byte, error) {

	// Determine the appropriate decoder from the decoded version

	decodeFunction := interpreter.DecodeValue
	if version <= 3 {
		decodeFunction = interpreter.DecodeValueV3
	}

	// Decode the value

	value, err := decodeFunction(data, &owner, nil, version, nil)
	if err != nil {
		return nil,
			fmt.Errorf("failed to decode value: %w\n%s\n", err, hex.Dump(data))
	}

	// Encode the value using the new encoder

	newData, deferrals, err := interpreter.EncodeValue(value, nil, true, nil)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to encode value: %w\n%s\n",
			err, value,
		)
	}

	// Encoding should not provide any deferred values or deferred moves

	if len(deferrals.Values) > 0 {
		return nil, fmt.Errorf(
			"re-encoding produced deferred values:\n%s\n",
			value,
		)
	}

	if len(deferrals.Moves) > 0 {
		return nil, fmt.Errorf(
			"re-encoding produced deferred moves:\n%s\n",
			value,
		)
	}

	// Sanity check: Decode the newly encoded data again
	// and compare it to the initially decoded value

	newValue, err := interpreter.DecodeValue(
		newData,
		&owner,
		nil,
		interpreter.CurrentEncodingVersion,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to decode re-encoded value: %w\n%s\n",
			err, value,
		)
	}

	equatableValue, ok := value.(interpreter.EquatableValue)
	if !ok {
		return nil, fmt.Errorf(
			"cannot compare unequatable %[1]T\n%[1]s\n",
			value,
		)
	}

	if !equatableValue.Equal(newValue, nil, false) {
		return nil, fmt.Errorf(
			"values are unequal:\n%s\n%s\n",
			value, newValue,
		)
	}

	return newData, nil
}
