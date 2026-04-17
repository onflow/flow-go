package environment

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/onflow/cadence/common"
	"github.com/stretchr/testify/assert"
)

// TestComputationKindValues hardcodes every ComputationKind constant and its expected numeric value.
// If an existing constant's value changes, the value assertion fails.
// If a constant is removed, the test fails to compile.
// If a new constant is added to MainnetExecutionEffortWeights without updating this test, the
// bidirectional check against that map fails.
func TestComputationKindValues(t *testing.T) {

	// Single hardcoded map of all known ComputationKind constants (both FVM and Cadence) to their
	// expected numeric values. Every constant is referenced by its Go symbol so that removing a
	// constant causes a compilation error.
	knownKinds := map[common.ComputationKind]common.ComputationKind{
		// Cadence ComputationKind constants (range [1000, 2000))
		common.ComputationKindUnknown:                          0,
		common.ComputationKindStatement:                        1001,
		common.ComputationKindLoop:                             1002,
		common.ComputationKindFunctionInvocation:               1003,
		common.ComputationKindCreateCompositeValue:             1010,
		common.ComputationKindTransferCompositeValue:           1011,
		common.ComputationKindDestroyCompositeValue:            1012,
		common.ComputationKindCreateArrayValue:                 1025,
		common.ComputationKindTransferArrayValue:               1026,
		common.ComputationKindDestroyArrayValue:                1027,
		common.ComputationKindCreateDictionaryValue:            1040,
		common.ComputationKindTransferDictionaryValue:          1041,
		common.ComputationKindDestroyDictionaryValue:           1042,
		common.ComputationKindStringToLower:                    1055,
		common.ComputationKindStringDecodeHex:                  1056,
		common.ComputationKindGraphemesIteration:               1057,
		common.ComputationKindStringComparison:                 1058,
		common.ComputationKindEncodeValue:                      1080,
		common.ComputationKindWordSliceOperation:               1081,
		common.ComputationKindUintParse:                        1082,
		common.ComputationKindIntParse:                         1083,
		common.ComputationKindBigIntParse:                      1084,
		common.ComputationKindUfixParse:                        1085,
		common.ComputationKindFixParse:                         1086,
		common.ComputationKindSTDLIBPanic:                      1100,
		common.ComputationKindSTDLIBAssert:                     1101,
		common.ComputationKindSTDLIBRevertibleRandom:           1102,
		common.ComputationKindSTDLIBRLPDecodeString:            1108,
		common.ComputationKindSTDLIBRLPDecodeList:              1109,
		common.ComputationKindAtreeArraySingleSlabConstruction: 1200,
		common.ComputationKindAtreeArrayBatchConstruction:      1201,
		common.ComputationKindAtreeArrayGet:                    1202,
		common.ComputationKindAtreeArraySet:                    1203,
		common.ComputationKindAtreeArrayAppend:                 1204,
		common.ComputationKindAtreeArrayInsert:                 1205,
		common.ComputationKindAtreeArrayRemove:                 1206,
		common.ComputationKindAtreeArrayReadIteration:          1207,
		common.ComputationKindAtreeArrayPopIteration:           1208,
		common.ComputationKindAtreeMapConstruction:             1220,
		common.ComputationKindAtreeMapSingleSlabConstruction:   1221,
		common.ComputationKindAtreeMapBatchConstruction:        1222,
		common.ComputationKindAtreeMapHas:                      1223,
		common.ComputationKindAtreeMapGet:                      1224,
		common.ComputationKindAtreeMapSet:                      1225,
		common.ComputationKindAtreeMapRemove:                   1226,
		common.ComputationKindAtreeMapReadIteration:            1227,
		common.ComputationKindAtreeMapPopIteration:             1228,

		// FVM ComputationKind constants (range [2000, 3000))
		ComputationKindHash:                       2001,
		ComputationKindVerifySignature:            2002,
		ComputationKindAddAccountKey:              2003,
		ComputationKindAddEncodedAccountKey:       2004,
		ComputationKindAllocateSlabIndex:          2005,
		ComputationKindCreateAccount:              2006,
		ComputationKindEmitEvent:                  2007,
		ComputationKindGenerateUUID:               2008,
		ComputationKindGetAccountAvailableBalance: 2009,
		ComputationKindGetAccountBalance:          2010,
		ComputationKindGetAccountContractCode:     2011,
		ComputationKindGetAccountContractNames:    2012,
		ComputationKindGetAccountKey:              2013,
		ComputationKindGetBlockAtHeight:           2014,
		ComputationKindGetCode:                    2015,
		ComputationKindGetCurrentBlockHeight:      2016,
		ComputationKindGetStorageCapacity:         2018,
		ComputationKindGetStorageUsed:             2019,
		ComputationKindGetValue:                   2020,
		ComputationKindRemoveAccountContractCode:  2021,
		ComputationKindResolveLocation:            2022,
		ComputationKindRevokeAccountKey:           2023,
		ComputationKindSetValue:                   2026,
		ComputationKindUpdateAccountContractCode:  2027,
		ComputationKindValidatePublicKey:          2028,
		ComputationKindValueExists:                2029,
		ComputationKindAccountKeysCount:           2030,
		ComputationKindBLSVerifyPOP:               2031,
		ComputationKindBLSAggregateSignatures:     2032,
		ComputationKindBLSAggregatePublicKeys:     2033,
		ComputationKindGetOrLoadProgram:           2034,
		ComputationKindGenerateAccountLocalID:     2035,
		ComputationKindGetRandomSourceHistory:     2036,
		ComputationKindEVMGasUsage:                2037,
		ComputationKindRLPEncoding:                2038,
		ComputationKindRLPDecoding:                2039,
		ComputationKindEncodeEvent:                2040,
		ComputationKindEVMEncodeABI:               2042,
		ComputationKindEVMDecodeABI:               2043,
	}

	// Verify each constant's numeric value matches the hardcoded expectation.
	for actual, expected := range knownKinds {
		assert.Equal(t, expected, actual, "ComputationKind %d has unexpected value", actual)
	}

	// Verify that every key in MainnetExecutionEffortWeights is present in knownKinds.
	// This catches new constants added to the weights map without updating this test.
	for kind := range MainnetExecutionEffortWeights {
		_, ok := knownKinds[kind]
		assert.True(t, ok, "MainnetExecutionEffortWeights contains ComputationKind %d which is not in knownKinds — add it to this test", kind)
	}
}

// TestPrintMainnetWeightsCadenceJSON is a test that prints MainnetExecutionEffortWeights
// as a Cadence JSON dictionary.
func TestPrintMainnetWeightsCadenceJSON(t *testing.T) {
	t.Skip("This test is only useful for preparing the cadence JSON for the transactions for updating the weights")

	type cadenceValue struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}
	type cadenceKV struct {
		Key   cadenceValue `json:"key"`
		Value cadenceValue `json:"value"`
	}
	type cadenceDict struct {
		Type  string      `json:"type"`
		Value []cadenceKV `json:"value"`
	}

	// Sort by computation kind ID for stable output.
	keys := make([]int, 0, len(MainnetExecutionEffortWeights))
	for kind := range MainnetExecutionEffortWeights {
		keys = append(keys, int(kind))
	}
	sort.Ints(keys)

	entries := make([]cadenceKV, 0, len(MainnetExecutionEffortWeights))
	for _, k := range keys {
		weight := MainnetExecutionEffortWeights[common.ComputationKind(k)]
		entries = append(entries, cadenceKV{
			Key:   cadenceValue{Type: "UInt64", Value: fmt.Sprintf("%d", k)},
			Value: cadenceValue{Type: "UInt64", Value: fmt.Sprintf("%d", weight)},
		})
	}

	out, err := json.MarshalIndent([]cadenceDict{{Type: "Dictionary", Value: entries}}, "", "  ")
	assert.NoError(t, err)
	fmt.Println(string(out))
}
