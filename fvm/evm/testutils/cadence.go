package testutils

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"
	"github.com/stretchr/testify/require"
)

func EncodeArgs(argValues []cadence.Value) [][]byte {
	args := make([][]byte, len(argValues))
	for i, arg := range argValues {
		var err error
		args[i], err = json.Encode(arg)
		if err != nil {
			panic(fmt.Errorf("broken test: invalid argument: %w", err))
		}
	}
	return args
}

func CheckCadenceEventTypes(t testing.TB, events []cadence.Event, expectedTypes []string) {
	require.Equal(t, len(events), len(expectedTypes))
	for i, ev := range events {
		require.Equal(t, expectedTypes[i], ev.EventType.QualifiedIdentifier)
	}
}
