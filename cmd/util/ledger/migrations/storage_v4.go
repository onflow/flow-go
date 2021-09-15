package migrations

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"runtime"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/state"
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
		migratedPayload, err := reencodePayloadV4(payload)
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
			if err := checkStorageFormatV4(migratedPayload); err != nil {
				panic(fmt.Errorf("%w: key = %s", err, payload.Key.String()))
			}
			result.Payload = migratedPayload
		}
		results <- result
	}
}

var storageMigrationV4DecMode = func() cbor.DecMode {
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

func reencodePayloadV4(payload ledger.Payload) (ledger.Payload, error) {

	keyParts := payload.Key.KeyParts

	rawOwner := keyParts[0].Value
	rawController := keyParts[1].Value
	rawKey := keyParts[2].Value

	// Ignore known payload keys that are not Cadence values

	if state.IsFVMStateKey(string(rawOwner), string(rawController), string(rawKey)) {
		return payload, nil
	}

	value, version := interpreter.StripMagic(payload.Value)

	err := storageMigrationV4DecMode.Valid(value)
	if err != nil {
		return payload, nil
	}

	// Extract the owner from the key and re-encode the value

	owner := common.BytesToAddress(rawOwner)

	newValue, err := reencodeValueV4(value, owner, string(rawKey), version)
	if err != nil {
		return ledger.Payload{},
			fmt.Errorf(
				"failed to re-encode key: %s: %w\n\nvalue:\n%s",
				rawKey, err, hex.Dump(value),
			)
	}

	payload.Value = interpreter.PrependMagic(
		newValue,
		interpreter.CurrentEncodingVersion,
	)

	return payload, nil
}

func reencodeValueV4(data []byte, owner common.Address, key string, version uint16) ([]byte, error) {

	// Determine the appropriate decoder from the decoded version

	decodeFunction := interpreter.DecodeValue

	// Decode the value

	path := []string{key}

	value, err := decodeFunction(data, &owner, path, version, nil)
	if err != nil {
		return nil,
			fmt.Errorf(
				"failed to decode value: %w\n\nvalue:\n%s\n",
				err, hex.Dump(data),
			)
	}

	// Encode the value using the new encoder

	rewriteTokenForwarderStorageReferenceV4(key, value)

	newData, deferrals, err := interpreter.EncodeValue(value, path, true, nil)
	if err != nil {
		//return nil, fmt.Errorf(
		//	"failed to encode value: %w\n%s\n",
		//	err, value,
		//)

		fmt.Printf(
			"failed to encode value for owner=%s key=%s: %s\n%s\n",
			owner, key, err, value,
		)
		return data, nil
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
		path,
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

var flowTokenReceiverStorageKey = interpreter.StorageKey(
	interpreter.PathValue{
		Domain:     common.PathDomainStorage,
		Identifier: "flowTokenReceiver",
	},
)

var tokenForwardingLocationTestnet = common.AddressLocation{
	Address: common.BytesToAddress([]byte{0x75, 0x4a, 0xed, 0x9d, 0xe6, 0x19, 0x76, 0x41}),
	Name:    "TokenForwarding",
}

var tokenForwardingLocationMainnet = common.AddressLocation{
	Address: common.BytesToAddress([]byte{0x0e, 0xbf, 0x2b, 0xd5, 0x2a, 0xc4, 0x2c, 0xb3}),
	Name:    "TokenForwarding",
}

func rewriteTokenForwarderStorageReferenceV4(key string, value interpreter.Value) {

	isTokenForwardingLocation := func(location common.Location) bool {
		return common.LocationsMatch(location, tokenForwardingLocationTestnet) ||
			common.LocationsMatch(location, tokenForwardingLocationMainnet)
	}

	compositeValue, ok := value.(*interpreter.CompositeValue)
	if !ok ||
		key != flowTokenReceiverStorageKey ||
		!isTokenForwardingLocation(compositeValue.Location()) ||
		compositeValue.QualifiedIdentifier() != "TokenForwarding.Forwarder" {

		return
	}

	const recipientField = "recipient"

	recipient, ok := compositeValue.Fields().Get(recipientField)
	if !ok {
		fmt.Printf(
			"Warning: missing recipient field for TokenForwarding Forwarder:\n%s\n\n",
			value,
		)
		return
	}

	recipientRef, ok := recipient.(*interpreter.StorageReferenceValue)
	if !ok {
		fmt.Printf(
			"Warning: TokenForwarding Forwarder field recipient is not a storage reference:\n%s\n\n",
			value,
		)
		return
	}

	if recipientRef.TargetKey != "storage\x1fflowTokenVault" {
		fmt.Printf(
			"Warning: TokenForwarding Forwarder recipient reference has unsupported target key: %s\n",
			recipientRef.TargetKey,
		)
		return
	}

	recipientCap := interpreter.CapabilityValue{
		Address: interpreter.AddressValue(recipientRef.TargetStorageAddress),
		Path: interpreter.PathValue{
			Domain:     common.PathDomainStorage,
			Identifier: "flowTokenVault",
		},
	}

	fmt.Printf(
		"Rewriting TokenForwarding Forwarder: %s\n\treference: %#+v\n\tcapability: %#+v\n",
		compositeValue.String(),
		recipientRef,
		recipientCap,
	)

	compositeValue.Fields().Set(recipientField, recipientCap)
}

func checkStorageFormatV4(payload ledger.Payload) error {

	if !bytes.HasPrefix(payload.Value, []byte{0x0, 0xca, 0xde}) {
		return nil
	}

	_, version := interpreter.StripMagic(payload.Value)
	if version != interpreter.CurrentEncodingVersion {
		return fmt.Errorf("invalid version for key %s: %d", payload.Key.String(), version)
	}

	return nil
}
