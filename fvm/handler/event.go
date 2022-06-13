package handler

import (
	"encoding/base64"
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func mustDecodeBase64(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

var (
	faultyTxOverride = map[flow.Identifier]struct {
		eventIndex uint32
		txIndex    uint32
		payload    []byte
	}{
		flow.MustHexStringToIdentifier("016003ece13ccb9fce32d7d93c4408395856becbacc9e0587d667b594645fc0a"): {
			eventIndex: 3,
			txIndex:    0,
			payload:    mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("502b84bf2d9edbe7e701cc54fd2c7a5e42d72b4936e7dc629635dcb640c5a9c3"): {
			eventIndex: 3,
			txIndex:    0,
			payload:    mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("358f50955d64b4e8cba98ac9d2b6042359def400405d318c209cca52e078e81b"): {
			eventIndex: 3,
			txIndex:    3,
			payload:    mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("502b84bf2d9edbe7e701cc54fd2c7a5e42d72b4936e7dc629635dcb640c5a9c3"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("ab57141ed1c02752be6588ecdfe05036c8b61c8aec0bcc3954c12ca8348aa2f8"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("460d475b646f488c2d01f7418d8a2748cabb89d8003b5b95e648a685aac4042c"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("0ccf3fc9d1193fb271c20d8a521dc57a17c2498526c337fc630d286a8236ba6e"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("6d620bc24a2dc0ba0f6402a71af5ddfe1fcd7cd76cc7c191d66099d3002406fe"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("a4a6ff0655559d689756db33dbc1237261c2552abe2f85566511b6274b072982"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("a7238782a6ae08385e7adcf7f800ca04ff4579e7e3bbe0101b0d16d66a9bcd04"): {
			eventIndex: 2,
			txIndex:    9,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("ba463e5705f2564f87d10dd6125377598b7b6807accb75213ec44f2407ba5652"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("8b0a7f5904dffdfd6645d25d5d5ed2338860e61f3f5ac23b306a229630025cfe"): {
			eventIndex: 2,
			txIndex:    2,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("984c3700a159b61362d056cbcfd4f60e1c188a2310f6cf8820627a3041c350ee"): {
			eventIndex: 3,
			txIndex:    3,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("47cf3b755b3f046650cb8b2b0a58a2f096f4656defb20915de2f077adf81cadd"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("e5e865aab3d15959d993592526cf14df5c3504fd859047364eb6e39758da4ceb"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("6000a1874e8809c87a64eeaabb29da8d79f7bf4d7e6229b21424ee49620fe364"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("92a2718ed2707008d6e44867160de1f1fec76024175cf40b3385fda6336635e9"): {
			eventIndex: 2,
			txIndex:    19,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("3e443a0eba10226356dc8fb6a3090085878936a199bec8517d3bdf9ac67f72b8"): {
			eventIndex: 3,
			txIndex:    9,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("ecb7724d212a3fd4b6e2bf4a56c7664a50e925ed4a094052c969c975f7a4ad52"): {
			eventIndex: 3,
			txIndex:    12,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("4f938a265ad1dd714b4f7d36b2032fe76ed9106ca05c0836ed232358e383ef23"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("ce58a6d48fa8ec3b451919fb74b0a86e4309745734b39413f4fcf28142436578"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("8c6e042c310291003d707933a70d00722a93892d24cfb9f0ea1b2c0717c1b349"): {
			eventIndex: 3,
			txIndex:    11,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("359d72465a7910af257e45213a9f677eb8b09aabf308b5b318b649d8d3c600fb"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("a18277cb25ad769fe2edf3189f12ba329a5701d36b8e211f28dbac9dcb34b55a"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("53ca7177cd9e46000dcbf38622402ed4991c4c12b1871139d14059825e3f0f92"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("2a26e028009d3828074d82e875626f4e465339dbdd99d41a9e97120cbe963cda"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("5671311745410a08b2608f6d9d450f57869dc2b037b2c88330b7ffb3abb26b69"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("ed8ccf985bd65adb633d6aa3932eb7d0a62b8a12e7419ce9eb23aa4d80c9944c"): {
			eventIndex: 3,
			txIndex:    3,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("00f04a7b23a853f82d59c8485a1259386e9c93817d1e1ef26aebad4faea4d14f"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("89934fc49e42c15458af54a2cfaf02a71e3edc0335564fb8e26dbf06ed94d363"): {
			eventIndex: 3,
			txIndex:    7,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("84ac1e990ab75ab4cb92d2c75c06c4a771e83c36f88b664439556e3b275d2662"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("9d31c36cb4600ac3364260b3c413e854c4109e3e34189be68b2413ff15604896"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("5bb567cbb2b57b9605f8b48c14bc04ccee62f041f655d0dbea44682df7f04c26"): {
			eventIndex: 3,
			txIndex:    11,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("614bb050494b4dc0f6ce7304cb62c85f2de261e1c19f8e3d31665eea91b2d242"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("b48a91d7068e5a655845384ac624fde73d2caac517d6b65b53fcf4706e439fe0"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("4ae262f5ccea7891707eb02d4611c38886930886db3d24e3236cdf836e398efb"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("3fc560b2c06306beddf73301c171389c1a219757ffa5c625897914ac318c2e06"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("f2bd0bbdf2a9ac4d870bfa12783b90b568459af9310d0c493e224a74a8e0530b"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("d6fe05786ebdc96b06a76fd0485e193660761a038f1a550874c8f0985fe43dd3"): {
			eventIndex: 2,
			txIndex:    17,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("c1f8dad08bef0985cf3e3b5a13d474e5c1862557880c6d9041fb8e7caa788c5e"): {
			eventIndex: 2,
			txIndex:    81,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("244f537c64f4dae966c28ca5dba19c5e94b67e31a0f6b40630e3039f79d62a53"): {
			eventIndex: 2,
			txIndex:    31,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("4e4a6759b379ddfbdb7234530ace27f391539efa518eaeb69744ea238d2abd53"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("42bba907eff30efcac83e7f50c147dc9cdb67d3855e6e707a957bac4b8123078"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("b6c3026ccf2a96123e7b45ff096149030ea6eaca6d54f2d65ad5b0deb06642c6"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("ebb42f5e584ec5a3183930983c9e3bd4fdb8f7ed6e1125ad28730c09b77432bf"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("75f47e7af035da2934f9d008eefe559a56f20a37560fd905149f99df813d4902"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("02cae2573f0afbbf2c5bcf4c1210da809aab2bad3a21224ecdd565ab914ce463"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("444162eec280a5f507debc018f2a618ee33e50d22ebc1665cf14e575f02e7cac"): {
			eventIndex: 3,
			txIndex:    46,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("2a706d9a247bdb91be4c3787270878e1a0e0021801318c404c27273b32c94c7e"): {
			eventIndex: 3,
			txIndex:    6,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("ae7ed4d2187727c2a2f0227dba4b4c73b399400439feb1963d0fc51e190dd759"): {
			eventIndex: 3,
			txIndex:    3,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("d213bd466244203639bf02bcb8a760cc492ad1a6001bf32d261a692cc06b4d30"): {
			eventIndex: 3,
			txIndex:    11,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("9f2fb6b65b6b177c222228b8306c298fec94f71837a6da5b490b1fa8153580ba"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("e51e3581dae4e4c00b3d6d4f7afe4b632e58f1eaed248502177114027ab6afe4"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("f86610a36bef85447c7a84598c4b355585a01f7bd0194adaefacf7b9c65b5006"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("7e5f237aa0ac032090f8a5746b69f26041df74910d6b50c19f5eb755445916ed"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000164"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDE2NCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("dbc9154480c82fe32e9546e9d3fe1258e584442ad50c215e26598eae5fde037d"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000172"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDE3MiJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("dc41ac68af5bd766b610978bfda66562c8ed12ae6849e4e81b98f853747f9c69"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("1adbf12ed69c1fa9bae6717683c7a3d6eeeb5216bdf700a7a0294cf96613d4cc"): {
			eventIndex: 3,
			txIndex:    30,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("0cf3d2b87a3ff01c3f3029c1c842537a732863966b2c0e82f9ff1d9619b3391f"): {
			eventIndex: 4,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000073"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDA3MyJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("1a3268ac610cac59d2b98fe91bccfbc9053d2ea44643e3d8111e4053b02240ea"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("6f5b2a61a6abcff0b7dade83e78348173b2518603e0f5a4b916d5088e9d2dc2d"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("2fd01a19c5d52f19564e87e38e143e1954d875b47b858bd6166c507715a97b5a"): {
			eventIndex: 3,
			txIndex:    24,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("66b1640e236858d1c0710e503628f25a90b49f45d22948724efe4617b0fa72f2"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("65c862e44f017c7e0fb9bdb4e157ecb0c7c9ddfd273e6fd22657e27e8c143c71"): {
			eventIndex: 3,
			txIndex:    8,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("d084ec37fd612119bee7b365905d858eb1209d54b15a0dae9b344bf341feb0bc"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("bbc6ce087efa958bf112dbef82759756d59bb37d4685bd51ef898dc7e89bbfa3"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("bc87ac15e3c0e0a05b6185088e9eeb33ac711654fde4012849250ddfef4f7b01"): {
			eventIndex: 2,
			txIndex:    8,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("363f90e2365aa774f7950a2ce6bd0adea1391778ae00f4d5776be89b5d63d971"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("e20d266ffc048c79f5ee411288601c8da7db688c4648f98451ad096962ccac2d"): {
			eventIndex: 2,
			txIndex:    42,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("5ca34352bc30f71beed0279859ab578b699a4b88db93197c4bf4cf65bf433bcb"): {
			eventIndex: 2,
			txIndex:    52,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("fa577fffbec909afbed6faf80d08fbdfb010371c58c7c9e3b74a05cfe6804d63"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("cea6abd29cb11fc22a7a390f840eb79b763b0604892c06bedfd72a09b0fffc34"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("0c3c3f25eccc33e40a76384d8bbb5f0e000e3970a63afe66044db75c01a7f252"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("d41bc869a0297cbf85c2aa79510bdc6feb3de7e28da3131b39a9d6ff499d9eb9"): {
			eventIndex: 3,
			txIndex:    11,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("68ffc4526cfe0cd04e727eea9a33f1bc5c3c17218272d2954037e3edfcc669eb"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("295e888c16dac5987cd3c038a6e487c01ca098137cb4b94beb401c119f77ca2d"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("11faf4ac0677e7764ba61b6770d7ee660e4424c9319f9984ad883bfeed7d7c2a"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("8b9f26c734ebe123997df89dbcb2fc6c20964b0b522411cc79ef2ac0625f6eb6"): {
			eventIndex: 3,
			txIndex:    5,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("3ab6b492dd33c63694a120a790f1eb24ddff86c47f16441ff32a9d4c4e94c28a"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("d44b8493206a595f6085a90b856e743d4c7c4ec1e8d077a7ed49ca2fd98e910c"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("832ce88160bfb104f41d4580f061be153899d9bdad0100539ac4425f17c6dc0d"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("928f7bc9d3413f7b9f2ae0722209d15db554b007b66854585ac5f6b144dbb41e"): {
			eventIndex: 3,
			txIndex:    3,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("6b8b0178b5804f1322a43c724838c720dfbafba43b171c4c270347ef9def3b34"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("549356dab60b911bedfa10b8df0426b1ef5d8cc371f7969e06c87d1e5daf8fee"): {
			eventIndex: 2,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("dd81158857ee8ddd1c2bfc1fbb08d642ab595a2156ea4ff3d1d03e3535646965"): {
			eventIndex: 3,
			txIndex:    32,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("b2e80761b8574c2199ec3cf50d1d4b8dfd144046b0d2e111bba1215ed93f602c"): {
			eventIndex: 14,
			txIndex:    2,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000155"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDE1NSJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("3e20cfdf51780872b9adb566abcd550d253e4a7567386e7e58a7fc33e8f19acf"): {
			eventIndex: 15,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000264"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDI2NCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("5158f7743e3d70feb76e7d620a64117fc33f001942f1c99dc7e23e0df6bddb85"): {
			eventIndex: 15,
			txIndex:    6,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000257"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDI1NyJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("f3438884d8b6b944d1e30f52886812f70b88570fd02668ecf072fbf1b3579c01"): {
			eventIndex: 2,
			txIndex:    30,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("69559039d05a67969d3ce62c5c5b380ffcc7b560f95d7fce00e60869efb60ad8"): {
			eventIndex: 3,
			txIndex:    13,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("7075502bdf368bc160a59e5ab4ee1df84ebd28e47d3ddd0522c224322dc2cd74"): {
			eventIndex: 11,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000201"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDIwMSJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("610320d3f274493bf625f1bc485c4328bed8f927f3817b6ad4fddc3ba51885d7"): {
			eventIndex: 3,
			txIndex:    3,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("68ef19b4360e33daa6cfabb8e319affe64d2afc20a7ff22fd89d43ee0e3d64fa"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("24e6b4372a9c3bcbb9f714c9ae14caf0fbf27e4d2ad7ff49a749a454252ac512"): {
			eventIndex: 11,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000201"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDIwMSJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("6ad398eac35a55b50aef4b09236585dec80e298504e01edcf3f118df7e94ca36"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("6f7223dd0e96f3b56cea37eb68adb28f58cb922ff0dae7624df95171247dbd6f"): {
			eventIndex: 3,
			txIndex:    15,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("e8d08470a12b48ecefa2c005dff356b2dc953bec64850e8e06119f47af46cd91"): {
			eventIndex: 3,
			txIndex:    4,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("8f29c5ac49e684483015e615e47624cf517634f58d8c508ab155ac86bff70578"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("5e688c4f78f78d566bdd7cbe0e5e8badf06a604c3ba00c9dc840eb8ac6b2cbfa"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("b6e880e7aed23ad6eaddb7b175336a50ae4906bf5e39b4b415f46ea9b6983740"): {
			eventIndex: 2,
			txIndex:    36,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("40c8c579be56dac85cfa4b68f777a06fdd98587f5f3825b2bc97ddbd6aa8c116"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("fc9e3bcdc7f7d9400a4530de915f540a0531e11093d096aa1d64352490f50748"): {
			eventIndex: 3,
			txIndex:    10,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("c70e44afb147f8ba0cfc330fc6e1f175e8a57a90bc440dcd23dbb8b5fb96d171"): {
			eventIndex: 3,
			txIndex:    8,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("6fade9e64624fb40ed71114d4c292354b0cc01e60bf8501c677ba727edb58e4c"): {
			eventIndex: 2,
			txIndex:    8,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("2ce064db0a95cf2376e033810ced9866a85012532d2f887a6d0986ac8e9968b6"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("0616f18ee70f951763b6d0f049b2b2aae7eb9731a5d2d73db5ea1cc27720846d"): {
			eventIndex: 2,
			txIndex:    7,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("c14c423bb1251097fa1becb987b05c32c174c9869c40d57c8f22a1ac2fcdcd2b"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("f1a49a4a93a8f704ef9a5d788cd69b666751acf1b387de66ea576c67e1548914"): {
			eventIndex: 2,
			txIndex:    2,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("65c83dc3fdff53b7f274f6ea68a1e3d334928d9c024f88bbe447f4180ac39333"): {
			eventIndex: 3,
			txIndex:    14,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("0f7732bd3054073157a37fb56497d7de4e6c3e9623d97e7952fe2b6a5fec7597"): {
			eventIndex: 2,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("1867fa269de90fe9c51409223dbafcd30e876a6cb80322311c3fcf3604c35ec9"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("1c19719011657ea394f710633c21bf74708e35903bdd46c3feb012f343e60686"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("c25b2d761ba16c226cdf9db696ca345a3963638cb66e5bc3f0f99cde610c8ada"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("5347dca5e6f62507776e4397c82af616ea4e4a73c37cf6e0ca7ce903b5f9926d"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("3a86f77566dd43c74184605f8a56b1f1b5f79cba02abce369239307624b22fc3"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("0809b7611e970e45af6b53f73000ec2371a91f5dd45c05389ed4738176cfe2cd"): {
			eventIndex: 3,
			txIndex:    12,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("1b21af60d29626b6b35ec7fdb691bbb50a0799fec35bf15ee5ee4bee87879d61"): {
			eventIndex: 3,
			txIndex:    11,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("3638c835877b2902e528038eb26fcf59a5cd611ce5baa312a472d81491a4d1cf"): {
			eventIndex: 3,
			txIndex:    25,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("2b68ebf90450f62d4ab3d78e24ba9fe37da66eddd20d89e15a9f612150d6774f"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("29da6f82895454d8a6f5eecfc8eb7cdedee1213d03fc31ed57015062a29302ce"): {
			eventIndex: 2,
			txIndex:    9,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("51b1dd6131d2c676604b346edae67b4bf1faa1d23b10cec7465b19bf36dd5ac9"): {
			eventIndex: 3,
			txIndex:    6,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("0b50fd9cc150f50701c11bb8b4634adf4af4e11ebfae8759c61d3896ad3381d4"): {
			eventIndex: 2,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("c0256b58a22c28cb466f9a0fbb7927a5328fdf96b703d0f10f2e018d34a04c10"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("3ad1ce83691c9b100ea20b91abf4bc0904dfb26cd82d4932e6356af388c1f640"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("b98ab4a7d0f5ed3fb8914ebcdc825d03dfc4248b61b01b501ada8a7b865e1501"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("564e580c3e66d3f96c0bb26f769cb00bacf3b73ee0ff1e221dcd5e6758693024"): {
			eventIndex: 2,
			txIndex:    13,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("076e68e6fe68871d4759c3119100b635d3890c4cc87516fabfadf427cd69f4d2"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("addbc1a9de3a6a88f14328af77f048a894a33365e01fccaaf7b28ca64100fa12"): {
			eventIndex: 2,
			txIndex:    3,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("8d71e950ab9e861c6a36b0df2140561682a9244bede415fb85183bfceac5e3ca"): {
			eventIndex: 3,
			txIndex:    3,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("8c4adf40dd39538485c4faf5550fc99cbbd262c5db1346247275f502a22c81ac"): {
			eventIndex: 2,
			txIndex:    34,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("70a62dda39459b127ac3e83a16b28e39499757f0f51f6609fe871ca07420df3b"): {
			eventIndex: 3,
			txIndex:    36,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("100a210c63c03fcf562fa81c08536dd20d0f121b2db8dc3695b23525fc8b65df"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("8723c9b02f87a2fac941273683205ec61c7ce1734f600865ea8a3c205d72582e"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("c449f96783482d54cf4db9d49db498d42c2afa60524563ec30183650d3e3f562"): {
			eventIndex: 2,
			txIndex:    16,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("c4facde968483a70761dc4c9d12abfb7e8c2122c628d40cc64fc6811c3e0b0c1"): {
			eventIndex: 3,
			txIndex:    13,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("b94b95f05dbc887d6af57abcb4b8bb7e37e4a5d5cdb139dc28244286956c14a0"): {
			eventIndex: 2,
			txIndex:    11,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("05fc838a8e67c5b6a49b4eb1841b63825b6c0f8c1b3b9a627f1dc75fdfb05ad7"): {
			eventIndex: 3,
			txIndex:    3,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("15828fa3ba4208370013017764fa3ba4b0a9515a1f80aabce5207f6f85383361"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("bb7e8f50e0bac288a1dc3d0565b071013bbf0f4efe6df8be7957420007e0686a"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("2f2a75d8475c5764fdd61bd45b04cb025e95243479837f1c165a19cd45e2a854"): {
			eventIndex: 3,
			txIndex:    22,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("5e5514211d761cc7d55f5d4beb158825d217d5a35b166e80091af13647e34c1c"): {
			eventIndex: 3,
			txIndex:    24,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("972666fdf60f5fe924a6984ed32db762675bceaac1c2669651ce05fc8d0d159d"): {
			eventIndex: 3,
			txIndex:    5,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("474718303a10b5dbaaa4065a0afefa825ff0c8de8a30b13be9d93ae7a13fcc8e"): {
			eventIndex: 3,
			txIndex:    23,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("86ef87c29b3dab22d7479d28de419d3531a6f246911c66f849697e48c7ab39f6"): {
			eventIndex: 3,
			txIndex:    24,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("0dc6c3cf696973b4adbcd4b12c957276b165e5bbecefe4e9e5b6df5fc6acddd4"): {
			eventIndex: 3,
			txIndex:    7,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("5eaae94f41a8dddd4d9daeb275f33cf337f5ef514e345d338343e6e57e09b263"): {
			eventIndex: 3,
			txIndex:    26,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000091"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDA5MSJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("a048ab5d1e18395b41770e67372834f892f8aae394c0278c7d0073ab5c7197e4"): {
			eventIndex: 4,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000111"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDExMSJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("59d368febf1c061c1191c233df4203b8ff71b679f87c651469899246a74d63eb"): {
			eventIndex: 3,
			txIndex:    27,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("2f6fc3560c82aea923a3a35fedcb1ac9be92168681049f4e4a79f48df9c5be53"): {
			eventIndex: 2,
			txIndex:    13,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000031"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAzMSJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("08a19b40efddea3b2fac47fc66dcc648989fe5a3fc9a908e6ccd84f800d759b4"): {
			eventIndex: 3,
			txIndex:    22,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000091"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDA5MSJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("f30cd8264ce9389e5913515160b4e93fd62d89811fda580f0475a2b64a1f307f"): {
			eventIndex: 11,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000160"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDE2MCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("3a5ea7371459ea3b918ba11bf3f863c902584fa7f8590fb5ab814dd5f879a1ef"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000031"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAzMSJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("1a519e50bc236e322cabce582e28131b18e5587a4b93a0df5fb5c2ab261b0945"): {
			eventIndex: 2,
			txIndex:    6,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000031"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAzMSJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("b20ecc422f31f2eb03b1d95fda8eaea89124fc19b5d9ddcb1590a9632fd147c1"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("e92a2cc1264bade3f1d8796172b7e532c56ef0232f1ff12632081734336ab858"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("2ae3e329779e4742dd18c85be24e6eebd38fabc45afec35b2188b4ff54bcb0ad"): {
			eventIndex: 2,
			txIndex:    3,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("ac11fe4123d5a778c345be0ecff0ef8fc28e3ba0cc687267bd0bb0b96373ccc9"): {
			eventIndex: 2,
			txIndex:    8,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("cd7c795ef6b279fdc148e9ae1138ed51c0153f4b0c7cf809fc145532e34584db"): {
			eventIndex: 3,
			txIndex:    8,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("9f98258089f8bdc968159c8c635da5c5628ab98fbdf98433e4eee0a8e3dd8a32"): {
			eventIndex: 3,
			txIndex:    6,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("5f3e76556a751bf290394a57d430f4ccb62c32be6f085607409f6ace067f284e"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("fffebdc9f1ad2c227e064a797369b35e7f02c0feb5a5359d4878810fdb06786c"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("d761d721d7eaf1679ebed2f745e7d75d9322602c48b12a6eb592777a407fd48c"): {
			eventIndex: 2,
			txIndex:    4,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("5713b93686a02ea33a46ff8dc4696edf9a3137bc6a22ecc512feed46fd59d2f5"): {
			eventIndex: 2,
			txIndex:    16,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("94cb3ddfe081859ed3d784095d90d0cda89526b734c7bad193ed5138ecb76caf"): {
			eventIndex: 3,
			txIndex:    3,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("211ab838a1fab3854ba9d68dde81e0b9d14831b3653fe8b39535a3da8e7ec9c1"): {
			eventIndex: 3,
			txIndex:    41,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("e0c98ce9f3bb37c907b84d422a77eb7e0723f308f5ee21a500411f179a5c0c76"): {
			eventIndex: 3,
			txIndex:    3,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("550ad3fcd93fb1790fddf662b960102f5f400fa605bbd22fdf93dff373e88a24"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("83e3db6ace58d2253ab1779c4675321692a23ce86f557ae84f1f57ed8533e2c1"): {
			eventIndex: 3,
			txIndex:    10,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("e7361f247027393bc294adf122952b35a66c753ba6fbfef3396742737da5841a"): {
			eventIndex: 6,
			txIndex:    51,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00001431"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMTQzMSJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("e41e34ca8a8a4ce1f9361f03fd77295dd8ba7756c6a633587d958460200a0723"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("f8251dc76922b50796e352c5bdb8d850e86a527f4a1c5f9abc9c1637589a77fc"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("920b0b8346520d5ce01903fe525629d7c57e082d6ebcb96991bdf68d7696a550"): {
			eventIndex: 3,
			txIndex:    27,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("7f2773df8bee4237f3de81e22f4298020c17e73561e78e55eab0297e1ce82f10"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("fb8f2dc08f95d3922cccedc5dfa8b95774286a29f5c85db4a53ab19d70e28cb5"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("d82728bc616a732709b1f45cadf670ff5116bd1e72b086ad2571d11b4c7c898b"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("358f50955d64b4e8cba98ac9d2b6042359def400405d318c209cca52e078e81b"): {
			eventIndex: 3,
			txIndex:    3,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("4d580e5547b5a5c86cc201e54a656fd49db50ea399d093ae1f60567b680cefec"): {
			eventIndex: 2,
			txIndex:    18,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000055"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDA1NSJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("6991856b98cc14ecabe9e06c8ca7d3980cf211950c6556cb070a9d674c61a9f9"): {
			eventIndex: 3,
			txIndex:    27,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("de5fb7411d96392b5d2e20fc158dbfa93689c2814cd40dc933b2357e769d3139"): {
			eventIndex: 3,
			txIndex:    15,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("6dff02b17ca8240c8c06a5630d931527cd972675fcb05950ffe0f3a099fde2ea"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("a900173428c3d6096f2b9f610cd556234e53b2655531b7350340b6476cfc5bb6"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("3fda350c2520da4130ac63cb2d68bcfb5e62707297da2b7c12691307d3e3f546"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("441630e1f5af54eaf9929f8b54db3757a229d3e3292bd6c01a8c20780ec53e9b"): {
			eventIndex: 3,
			txIndex:    2,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("e778c6712a8bc50cc9d3a7faf7f72d79b84aba26f4d62fc6ef7f471ad4046764"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("7aa898f891dd800fd8dc2ce8b08954117ede9c31fd6425f23ca1450be00d05a4"): {
			eventIndex: 3,
			txIndex:    2,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("652faf65e0d17b44baf207372279bfb44691b131f8d7e41d7df65322f13c0e20"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("c85a5ab319c162fb2acace8ea5ad53af642f43471f41e198f83ef0f402072085"): {
			eventIndex: 3,
			txIndex:    71,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("80dbef878600fb84e1d47983b4cc62d9b8394c320a1db684b5ab16a078a7e56b"): {
			eventIndex: 3,
			txIndex:    36,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("02e65cd9c6a7b6eec337758b37eb5cb86831d4b229ac839384c4350c1b5ca139"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("55364697645c074fe833696c86b4d3174b26112580bca76595cf1d0eeb2123c3"): {
			eventIndex: 3,
			txIndex:    16,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("a746c85bdcf99d384110487e994289548fb5e2017901b17c73ec802d0597e7aa"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("9ea01c59b1a80491545343b148a7849dbc370ea87d56ecc5e804b0a2a719f8b1"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("c92a84f9f0298c94275d7c086876a46940fcabd55d7c7ca5274e9c1423ed890a"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("8f07c4a4f944a867e6d0f4d4d65ea5f7a8a95c0f2d5e9266050e67128e808bee"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("7420a95a5fb52e5ed156dd0e896807c91db13e669b8bae93a356f8272b12fa7c"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("e4685a8109945377185c9462c336a7f7214a39592ee28f0fceff591fd5e4e20d"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("c014d7096a2f04fc8201d532190ec183bc96769b103210b0f6f588aa80d2f8c9"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("fa04d2a2e44355f30962953965038b2ec2631ccf56dce123f9b12a1c8d967474"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("fecd9ebf2eeed5fe61f16f766e73f04891160f4587ec4c41465bb5b3d31f9086"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("c4279721d241cc1c6934822aaa83c707b9a22b447c4968fa4cb3ad1de3b817ed"): {
			eventIndex: 2,
			txIndex:    15,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("188fb2bfa9b303b133af9529eccd117f3a15472d928417ea699ec44fbf7d9e5f"): {
			eventIndex: 3,
			txIndex:    4,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("5758caaad5b25063423e32dc723e2fb7aca97499a0101124ceb3bd354354c1af"): {
			eventIndex: 3,
			txIndex:    10,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("5004786682f7be74091ed63625c29eec14ad058eb0e58a000cb755c94c977f56"): {
			eventIndex: 3,
			txIndex:    10,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("6a2971ffb84490ea239ced2f94768dd692e43875b3d5ee1f724ed6b7f0a5a0f6"): {
			eventIndex: 3,
			txIndex:    47,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("beca6234ee2d0b1cd83feb16c1121692bf79f7d8ba5a9113901bac290c4c6bba"): {
			eventIndex: 3,
			txIndex:    4,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("17ce5e43c3ec6f33cc68e4afd75db0d603bb906fdc73b865f2f838a935b6d0fa"): {
			eventIndex: 3,
			txIndex:    16,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("cb2197a55e76c8610e8bd8de5a70d1fe17cc17aeedd27afcb0806e51f5224a36"): {
			eventIndex: 3,
			txIndex:    9,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("93cd936ae6f0911e4a43f78087757df937177a65789053c2b6c61be8ccdc15a6"): {
			eventIndex: 3,
			txIndex:    22,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("90e41b09b4c98d6b0f02642c86afe4c6edc3f3606f8b671aa2bebd3fa5d36dc7"): {
			eventIndex: 3,
			txIndex:    35,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("9b1ecfcac15e41da1fc0f8c2f3b615599153656536e23135b697347764055ae8"): {
			eventIndex: 3,
			txIndex:    11,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("1427fec4cde1bdc0f670fbd7c3ad1eb1a1ee11b6412b1e4f4351e4e92d7e080f"): {
			eventIndex: 2,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("de03e19db8c227f3b6e85e1036d0b5c72873688c57d8b98bb3c0b738c59940e6"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("b3b33ddb52014a9668bdb6533d6c808e070c0c969726fb1118828608951204ca"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("c734899e59e35abe42476553108a6189c8ce023fd64a4025d352727a567837ba"): {
			eventIndex: 3,
			txIndex:    2,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("5ddcf8e1d5d42c5a8e0485b23b450d5e7e10b7e532344ed2a591176b39140efd"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("0b82a6807259c221d58b815f1d90e6458561399f28ced6482a15b90c350c35eb"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("651fb198708cecb81f57a182f7030d516463814c542afb97db760b34d898154c"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("febe9b8666bc39a73f185a1527864801e65b0bb5556c9079985898c36a4d2fce"): {
			eventIndex: 3,
			txIndex:    21,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("0161a13925e0002c0d3e0e527b040930e1d21732e0e297406232a24956219ed9"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("59235d42f08364e41742ccb890e51ff64f4af280f021ea12d092e4942d09e8de"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("3e17b3decd35201fb26ff6011ade8f6ce51ed8e86aa7e244a3b74de4439b24fd"): {
			eventIndex: 3,
			txIndex:    43,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("a41888edef3725a8c7d3cc11b856aba8c98333c7b58e15e6dba43349b495406c"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("544c92419dcffca2905e17fd2acc72b8e74a31e5c94db1ff905a8bc4824b407f"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("dd8456d0ad58577857b7105eb85dfd8d00869ac7e02970c870e1c2fe2aa0df48"): {
			eventIndex: 3,
			txIndex:    3,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("e8ac0fe49f8a683bbc9cf333fa82c16455231094433f13dbc5e51bcba7fdd373"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("ddbaa3e02be397890a273394a293220ce3e8dfe29f060f09da76b8e3301740c1"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("f259e0f765484fbed10fd026f15738826fe3cbae49d57500a88b1391e919530d"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("4e2da87b15bebcd6ff3082415ff3aa75a23c5b3f9fabe67dfec9058aa614236f"): {
			eventIndex: 3,
			txIndex:    37,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("a34d88c5b3185bc4e863c53e12b9ddf4861d6597b7d512234afb730a4869936e"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00010000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDEwMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("b542bbc75ef6267e022519a89708ce499f9d707304579baf53d88ecf9e95e735"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("cbbeaa306f008a9d0ce802878e6a676c400c2b9e9baab5e4f360383bf70f54bf"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("0f26900068871167deed9cb2928f82576be62e9fe2e5c6fb38593cc3e1dbe883"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("1dbdb548efb15712fe767d845889be5662b23994813e923ef6fa4ad690466ec6"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("6508579ce83934519ba65c73e67f5c1c3525252ecfbbc2b8dda1e76da3641c27"): {
			eventIndex: 3,
			txIndex:    28,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("d9fdc00773849a8ccb437342a7454119b80380f54a59423c321536d1992095e0"): {
			eventIndex: 3,
			txIndex:    15,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("74bf2fccc5ca2fc562b8496c7894f3e2dac32e88ca3128508ceaed06b30256a7"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("f964894e592278ca7dcdc39288105ee14a8d2c88cc8870951d34c12c698355e8"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("3e81db5ecfe9ef809c139d68c8c41f3967eddbcb99465eaa9fbb12638f66e8cb"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("84d6269bf315119b6abd0d87810857b0529236d52063e5e83cf1ebfabe5f5911"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("87b9ecbc5e305b8089c6b49b603d1e203811aa808ef3ebf3169c0307b2919ca2"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("a6b51c71b61b40ac150c5313d5bb5be98f10502bc761130c0436789a55c986b8"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("4e375399a71566628ceaaf7168dfcafd860b481bf34a3d3daab42b9654e8d6ed"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("bab94b3aee2e0d28411ff64fcdcc2880186df13d5c5c4c3c8f9d7470404af17c"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("8b55409b81148b387bd14cb88eeca0c23713c5bedc3828a6c887411cce77cb8e"): {
			eventIndex: 2,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("21859ca05774cc60161a5acdfde606cf32541ec84698f839171ad4f4ffa02ee6"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("16d5b2860ab52be9fe1cea8e40325368700b75906825d72bd195337b7efe81a8"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("36a7ae72ee9e5e3f24405bcdc73a018f644ed9aa35157a96f4ca9929c59f53bc"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("5f819e0d598b984662ba79417949389c8cfaf311ae9fe9812a9929bbd2d48a45"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("a341ba355ed03b49be9903de2525529b0c5c56a8cd563e46821f998ab948bad0"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("8d628231fdd21b20ecac6d4fa7d18c2fecda4c63cd7ac1b545a3115b5333ffab"): {
			eventIndex: 2,
			txIndex:    2,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("26100f0fb19859eac17b8689b8ddcb50d92c699956d1545107bef4598f009ee9"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("9fb720138e5157496cb8bc531bd70ee79e755aae936c499f19855fed0c6b6c05"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("b9b11ea3b2950665f0fb12b365d3e038069f89dea86e89c6f6d745e3f770ddf0"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("c4eef7689da707e6e730853904d272369be18c7b09ad779b8daf2f8ca46eaeb2"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
		flow.MustHexStringToIdentifier("588349cbce6b877ea6f64e6de1d385c2b829ad7c6fdc0fc56af8f351d1c4e3c3"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("f83a7f28ef0cc3e66cc0e46da31c44ec9cdd01fc8086ba633218a9ca8f76800e"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("846556dd9ea82cdabfb50630f0e790982caa9c33d8054d7749e1091a513e0fdd"): {
			eventIndex: 2,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("484975251231596b6b9d1910ec758aec16a3107fb4ffd8ceb15894935f28941c"): {
			eventIndex: 2,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("188ba797ce9e76140d46a90b187428da42d888ae02fdc387be605fa48bab6c16"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("f0f222e775e9de9c2e1431b78966c244532984b9a3984143bf0b2b93350d917c"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("d0a9b8dc900ddef873a9bc8e6f6b5cb6af60ef90bf6915beb201309174234b79"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("4d98b07957ee41549dc3bd7b424f299149b86720e47a89dbf6b25fb383f8e2be"): {
			eventIndex: 2,
			txIndex:    71,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("0a5d6405a4da0ccca2fece14325daa5d437ddcbc4b8023b1ba3b59a1698c7e3d"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("8f31fba782b682d84c8cb60fa1b5dd3aa126afcf481f96e1709b5a613d879df2"): {
			eventIndex: 3,
			txIndex:    79,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("715e6170f934d029d402c635e6863f0e7c27bba65a94d793bd78ec3ed4084403"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("633cf4960dd7f928310563c2fb9b637044f9fd289c19b00770c45caad54bf7ed"): {
			eventIndex: 3,
			txIndex:    32,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("c382c9d790bd1542250dbfe603480b80ef3d5f5d2d0fef7619a5f5f791a2aa54"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("f7e73c88b3cd25ab8ebf1e80ed4dddba2855393b128bb83db4a62400564a27ac"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("96eab281a074caac7fb1b0c350b226c6c4143c344c655215c91678638284a3b1"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("aa325f901e1643b20b5f03d1dc17025b6b90a6bce3a68d0282bbc6afb6601d3b"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("31c8d16fc646b7322165848755104a63cd52b5aceccb154b0aa8c8aa774b1cb3"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("1ccc4252d426bcfa128d4d44f1a56fd2db43513f8db6186622536269b890d630"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("3e7a12e3e67cecd2afc3cb63a722ef6fbaddca6bb0b634439ba54cb0a2d85f3e"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("0db0c6a3f2c5bfd27acb6f1b68e2e3f8e5e0e3c6aa012a23035b1c4d9a2657e0"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("5fb383055ff5ae57020e4fe930c7065dc2b930cfb974c54000fd44ff844abbdf"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("db72a0dc2fc99ddb1139b81e05d542b6e7e2bb457ec0a2fc991c7ce71f5ff380"): {
			eventIndex: 3,
			txIndex:    95,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("9929c973f452301ba4fa6ca990fd46213b1c5a1704734c080c5869c109c6b59e"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("c096fa605e918e320a6162502b82fc10782e86ed96aaf74eb6176cbe5b10dd61"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("806c1cb85c310439e0c2c63a5ab2d4d894e6a351b69d9edb7d9a9c46d0331acd"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("c1e6e4ed8eb16e3758c99b770593e80cb627ed9afacb633527928c1ca75c3929"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("c95f2eff4e77cef718324e3cc0def28c27c8c95f5474e18ae58656ba7df4f0e1"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("523046baaf8a29143de46344ce413e5d6d2be166a64565374c67ab156f9478a7"): {
			eventIndex: 3,
			txIndex:    4,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("526a8824cb67923fa73c2be49560dba90ceb41131f15d19e7bba3dd85a41dfbf"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("4d57f6e424a61cb204402c38ed1b5b80fda86bd589054f31e4d320e64047db28"): {
			eventIndex: 3,
			txIndex:    48,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("697b9e39a469044ccf27d0503c7d507622338ba4c2714bac6719a23d9c951886"): {
			eventIndex: 3,
			txIndex:    103,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("f4400abec8a72641458d0126d359e017f59d83bf0cd9807775f70bfb6a9039af"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("1f890afe677e807320ba43f7c042a9d83e634aba747e8a4bd2dd9f4a72ee73cc"): {
			eventIndex: 3,
			txIndex:    1,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("75fa09f6d4af2be9411a2d2a1629880d5e331a5b248bdc2cd6c53ea11975e249"): {
			eventIndex: 3,
			txIndex:    9,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("b4436848d6fa16594b09f8a24b16b198d65e303c9762116cf67c8e7b63b007a0"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("ec7462bb146357947ff24973a9ddb3fa8d61dc5fed8044c7880208de3e178490"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("b5102ff8b5b24dbe2eec6d7241de3aea4a2154741ab7a6df39fcf726fb692cb6"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("8d70682b7ca0094bc6ddd4444e53df5d617a2132a0871f2226f2936a869feb00"): {
			eventIndex: 3,
			txIndex:    2,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("672a3d58080d49f4ce83c8d8d97796c9293fb9547aed25c1340ba74520f8faf5"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("e19446433718a31768ff9353499632fa49ca6b01169a76f1b806653b2d564e0f"): {
			eventIndex: 3,
			txIndex:    47,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("8b56e6a49cb4785c954f574cc74bd7165479a345aa7ed1032e4212fbfbab4af2"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("ae75c0b350b5e128a84a0853cef3e02f009736aa7d562c3b5dd0d5843f608c33"): {
			eventIndex: 3,
			txIndex:    4,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("e673f061600e78e2f284f1b10323dc966ba5500de83b0a3a4608dfca9172a6cd"): {
			eventIndex: 2,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("084baeaf58b70647f43e8f56f376d03dab1833d6343bdc90f49c54bd8a5ba87a"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("191fef451dc33e67004f7e91e1d1d8c7071d3bbd8b88a52a6519aa788f264cb2"): {
			eventIndex: 3,
			txIndex:    16,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("704ee38557fc97d74f0ab3628131e1741a0431a946193fb4c0f9dbf24f05bc2d"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("e2eda274de4423c979fb689c6f1bedf375b810c4f0e9f9f4ea76fb8c0c0da6a8"): {
			eventIndex: 3,
			txIndex:    14,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("adc2a2f44d1abee990537288561a27bc51799f6fc3549a5af48449a345072c00"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("d40092b4dd34021fc1139f5f00aa27d7af71259a0621ddd2b79a9851df0e8664"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("fb307c12a9b339b869fc50d39289113c436a34e9f55bce88e30f7498166d13e4"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("6372577062a048943dd3a6b4452d1fd7c78cde17c5df8ea7633deed3a3d17405"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("7139f90272395f13a328e611c29c5c7dbbd39b8b4dcb9be96355d634a3aa63f1"): {
			eventIndex: 3,
			txIndex:    112,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("635359d431a60dc60fb7fdc60247d0bc6574a7822dd737cedda68ec115641c57"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("cab91d6327141431ac40d4d1e151da921f0d7a19fa211322cd9cd574ad2e3703"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("5e59b62a53306d9496798f1d2979592af68fb70a3e6c85406969891412ba5c81"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("b6589e1ee7470d96355f9630804ee8d44fbef4172cfa3cf709919bb3dca5ff20"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("398b4a3b7637a4d01eca43fce1bf587cc02f736f8c93b38200b90e499fa94fce"): {
			eventIndex: 3,
			txIndex:    152,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("e4374c6bc87af357707ec10a5ba604eb18186059a774e6dec36b59b144e5c785"): {
			eventIndex: 3,
			txIndex:    84,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("b1fb71d9479ad3160c9ac2ba96cfbdabb911750d5949b3c90b3738bd9691cfc9"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("96135d37516fb88841ba271fa47624c1cd7e0a3214768d6edefe2deaf2efc57c"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("0baa0e4306a166235eaa8d8c3c53798261d791f362f3a343ef83134175d96e77"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("59869b9cbd02385d15bf17522e15a3c9cd83c62f5481292f73388c9d2402e56a"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("30d91523f830e902071d5b2bc0399d6df8e50d92235f8e7ed4ebf16a5c1a0eaa"): {
			eventIndex: 3,
			txIndex:    105,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		}, flow.MustHexStringToIdentifier("ee091a7252f4e1d9ba2f4bb07f82880a496d4f571c03e500607020b693fe44fa"): {
			eventIndex: 3,
			txIndex:    0,
			// {"type":"Event","value":{"id":"A.912d5440f7e3769e.FlowFees.FeesDeducted","fields":[{"name":"amount","value":{"type":"UFix64","value":"0.00001000"}},{"name":"inclusionEffort","value":{"type":"UFix64","value":"1.00000000"}},{"name":"executionEffort","value":{"type":"UFix64","value":"0.00000000"}}]}}
			payload: mustDecodeBase64("eyJ0eXBlIjoiRXZlbnQiLCJ2YWx1ZSI6eyJpZCI6IkEuOTEyZDU0NDBmN2UzNzY5ZS5GbG93RmVlcy5GZWVzRGVkdWN0ZWQiLCJmaWVsZHMiOlt7Im5hbWUiOiJhbW91bnQiLCJ2YWx1ZSI6eyJ0eXBlIjoiVUZpeDY0IiwidmFsdWUiOiIwLjAwMDAxMDAwIn19LHsibmFtZSI6ImluY2x1c2lvbkVmZm9ydCIsInZhbHVlIjp7InR5cGUiOiJVRml4NjQiLCJ2YWx1ZSI6IjEuMDAwMDAwMDAifX0seyJuYW1lIjoiZXhlY3V0aW9uRWZmb3J0IiwidmFsdWUiOnsidHlwZSI6IlVGaXg2NCIsInZhbHVlIjoiMC4wMDAwMDAwMCJ9fV19fQo="),
		},
	}
)

// EventHandler collect events, separates out service events, and enforces event size limits
type EventHandler struct {
	chain                         flow.Chain
	eventCollectionEnabled        bool
	serviceEventCollectionEnabled bool
	eventCollectionByteSizeLimit  uint64
	eventCollection               *EventCollection
}

// NewEventHandler constructs a new EventHandler
func NewEventHandler(chain flow.Chain,
	eventCollectionEnabled bool,
	serviceEventCollectionEnabled bool,
	eventCollectionByteSizeLimit uint64) *EventHandler {
	return &EventHandler{
		chain:                         chain,
		eventCollectionEnabled:        eventCollectionEnabled,
		serviceEventCollectionEnabled: serviceEventCollectionEnabled,
		eventCollectionByteSizeLimit:  eventCollectionByteSizeLimit,
		eventCollection:               NewEventCollection(),
	}
}

func (h *EventHandler) EventCollection() *EventCollection {
	return h.eventCollection
}

func (h *EventHandler) EmitEvent(event cadence.Event,
	txID flow.Identifier,
	txIndex uint32,
	payer flow.Address) error {
	if !h.eventCollectionEnabled {
		return nil
	}

	var payload []byte

	for faultyID, faultyData := range faultyTxOverride {
		if txID == faultyID {
			fmt.Printf("MAKS matching tx %x", txID)
			if h.eventCollection.eventCounter == faultyData.eventIndex && txIndex == faultyData.txIndex {
				fmt.Printf("MAKS matching indices %d %d setting payload to %x", h.eventCollection.eventCounter, txIndex, faultyData.payload)
				payload = faultyData.payload
			}
		}
	}

	if len(payload) == 0 {
		var err error
		payload, err = jsoncdc.Encode(event)
		if err != nil {
			return errors.NewEncodingFailuref("failed to json encode a cadence event: %w", err)
		}
	}

	payloadSize := uint64(len(payload))

	// skip limit if payer is service account
	if payer != h.chain.ServiceAddress() {
		if h.eventCollection.TotalByteSize()+payloadSize > h.eventCollectionByteSizeLimit {
			return errors.NewEventLimitExceededError(h.eventCollection.TotalByteSize()+payloadSize, h.eventCollectionByteSizeLimit)
		}
	}

	flowEvent := flow.Event{
		Type:             flow.EventType(event.EventType.ID()),
		TransactionID:    txID,
		TransactionIndex: txIndex,
		EventIndex:       h.eventCollection.eventCounter,
		Payload:          payload,
	}

	if h.serviceEventCollectionEnabled {
		ok, err := IsServiceEvent(event, h.chain.ChainID())
		if err != nil {
			return fmt.Errorf("unable to check service event: %w", err)
		}
		if ok {
			h.eventCollection.AppendServiceEvent(flowEvent, payloadSize)
		}
		// we don't return and append the service event into event collection as well.
	}

	h.eventCollection.AppendEvent(flowEvent, payloadSize)
	return nil
}

func (h *EventHandler) Events() []flow.Event {
	return h.eventCollection.events
}

func (h *EventHandler) ServiceEvents() []flow.Event {
	return h.eventCollection.serviceEvents
}

type EventCollection struct {
	events        flow.EventsList
	serviceEvents flow.EventsList
	totalByteSize uint64
	eventCounter  uint32
}

func NewEventCollection() *EventCollection {
	return &EventCollection{
		events:        make([]flow.Event, 0, 10),
		serviceEvents: make([]flow.Event, 0, 10),
		totalByteSize: uint64(0),
		eventCounter:  uint32(0),
	}
}

func (e *EventCollection) Child() *EventCollection {
	res := NewEventCollection()
	res.eventCounter = e.eventCounter
	return res
}

// Merge merges another event collection into this event collection
func (e *EventCollection) Merge(other *EventCollection) {
	e.events = append(e.events, other.events...)
	e.serviceEvents = append(e.serviceEvents, other.serviceEvents...)
	e.totalByteSize = e.totalByteSize + other.totalByteSize
	e.eventCounter = e.eventCounter + other.eventCounter
}

func (e *EventCollection) Events() []flow.Event {
	return e.events
}

func (e *EventCollection) AppendEvent(event flow.Event, size uint64) {
	e.events = append(e.events, event)
	e.totalByteSize += size
	e.eventCounter++
}

func (e *EventCollection) ServiceEvents() []flow.Event {
	return e.serviceEvents
}

func (e *EventCollection) AppendServiceEvent(event flow.Event, size uint64) {
	e.serviceEvents = append(e.serviceEvents, event)
	e.totalByteSize += size
	e.eventCounter++
}

func (e *EventCollection) TotalByteSize() uint64 {
	return e.totalByteSize
}

// IsServiceEvent determines whether or not an emitted Cadence event is considered
// a service event for the given chain.
func IsServiceEvent(event cadence.Event, chain flow.ChainID) (bool, error) {

	// retrieve the service event information for this chain
	events, err := systemcontracts.ServiceEventsForChain(chain)
	if err != nil {
		return false, fmt.Errorf("unknown system contracts for chain (%s): %w", chain.String(), err)
	}

	eventType := flow.EventType(event.EventType.ID())
	for _, serviceEvent := range events.All() {
		if serviceEvent.EventType() == eventType {
			return true, nil
		}
	}

	return false, nil
}
