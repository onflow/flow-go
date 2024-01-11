package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// transactionDataCollector collects data from a transaction execution logs.
type transactionDataCollector struct {
	TimeSpent              map[string]uint64
	MemAlloc               map[string]uint64
	ComputationIntensities map[string]map[string]uint64
	MemoryIntensities      map[string]map[string]uint64
	ComputationColumns     map[string]struct{}
	MemoryColumns          map[string]struct{}
	ComputationUsed        map[string]uint
	MemoryUsed             map[string]uint
	TransactionNames       map[string]string
}

type txWeights struct {
	TXHash                 string            `json:"tx_id"`
	LedgerInteractionUsed  uint64            `json:"ledgerInteractionUsed"`
	ComputationUsed        uint              `json:"computationUsed"`
	MemoryUsed             uint              `json:"memoryEstimate"`
	ComputationIntensities map[string]uint64 `json:"computationIntensities"`
	MemoryIntensities      map[string]uint64 `json:"memoryIntensities"`
}

type txDoneLog struct {
	TXHash        string `json:"tx_id"`
	TimeSpentInMS uint64 `json:"time_spent_in_ms"`
	MemAlloc      uint64 `json:"memory_used"`
}

func newTransactionDataCollector() *transactionDataCollector {
	return &transactionDataCollector{
		TimeSpent:              map[string]uint64{},
		MemAlloc:               map[string]uint64{},
		ComputationIntensities: map[string]map[string]uint64{},
		MemoryIntensities:      map[string]map[string]uint64{},
		ComputationColumns:     map[string]struct{}{},
		MemoryColumns:          map[string]struct{}{},
		ComputationUsed:        map[string]uint{},
		MemoryUsed:             map[string]uint{},
		TransactionNames:       map[string]string{},
	}
}

func (l *transactionDataCollector) Write(p []byte) (n int, err error) {
	if strings.Contains(string(p), "transaction execution data") {
		w := txWeights{}
		err := json.Unmarshal(p, &w)

		if err != nil {
			fmt.Println(err)
			return len(p), nil
		}

		l.ComputationIntensities[w.TXHash] = w.ComputationIntensities
		l.MemoryIntensities[w.TXHash] = w.MemoryIntensities
		l.ComputationUsed[w.TXHash] = w.ComputationUsed
		l.MemoryUsed[w.TXHash] = w.MemoryUsed

		for s := range w.MemoryIntensities {
			l.MemoryColumns[s] = struct{}{}
		}
		for s := range w.ComputationIntensities {
			l.ComputationColumns[s] = struct{}{}
		}

	}
	if strings.Contains(string(p), "transaction executed successfully") || strings.Contains(string(p), "transaction executed failed") {
		w := txDoneLog{}
		err := json.Unmarshal(p, &w)

		if err != nil {
			fmt.Println(err)
			return len(p), nil
		}

		l.MemAlloc[w.TXHash] = w.MemAlloc
		l.TimeSpent[w.TXHash] = w.TimeSpentInMS
	}
	return len(p), nil

}

// Merge merges the data from the other collector into this one.
func (l *transactionDataCollector) Merge(l2 *transactionDataCollector) {
	for k, v := range l2.TimeSpent {
		l.TimeSpent[k] = v
	}
	for k, v := range l2.MemAlloc {
		l.MemAlloc[k] = v
	}
	for k, v := range l2.ComputationIntensities {
		l.ComputationIntensities[k] = v
	}
	for k, v := range l2.MemoryIntensities {
		l.MemoryIntensities[k] = v
	}
	for k, v := range l2.ComputationColumns {
		l.ComputationColumns[k] = v
	}
	for k, v := range l2.MemoryColumns {
		l.MemoryColumns[k] = v
	}
	for k, v := range l2.ComputationUsed {
		l.ComputationUsed[k] = v
	}
	for k, v := range l2.MemoryUsed {
		l.MemoryUsed[k] = v
	}
	for k, v := range l2.TransactionNames {
		l.TransactionNames[k] = v
	}
}

var _ io.Writer = &transactionDataCollector{}

func computationIntensityName(s string) string {
	n, ok := computationIntensityNameMap[s]
	if !ok {
		return s
	}
	return n
}

var computationIntensityNameMap = func() map[string]string {
	m := map[string]string{}
	i := 1
	m[strconv.Itoa(1000+i)] = "Statement"
	i++
	m[strconv.Itoa(1000+i)] = "Loop"
	i++
	m[strconv.Itoa(1000+i)] = "FunctionInvocation"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "CreateCompositeValue"
	i++
	m[strconv.Itoa(1000+i)] = "TransferCompositeValue"
	i++
	m[strconv.Itoa(1000+i)] = "DestroyCompositeValue"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "CreateArrayValue"
	i++
	m[strconv.Itoa(1000+i)] = "TransferArrayValue"
	i++
	m[strconv.Itoa(1000+i)] = "DestroyArrayValue"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "CreateDictionaryValue"
	i++
	m[strconv.Itoa(1000+i)] = "TransferDictionaryValue"
	i++
	m[strconv.Itoa(1000+i)] = "DestroyDictionaryValue"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "EncodeValue"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "STDLIBPanic"
	i++
	m[strconv.Itoa(1000+i)] = "STDLIBAssert"
	i++
	m[strconv.Itoa(1000+i)] = "STDLIBRevertibleRandom"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "_"
	i++
	m[strconv.Itoa(1000+i)] = "STDLIBRLPDecodeString"
	i++
	m[strconv.Itoa(1000+i)] = "STDLIBRLPDecodeList"

	i = 1
	m[strconv.Itoa(2000+i)] = "Hash"
	i++
	m[strconv.Itoa(2000+i)] = "VerifySignature"
	i++
	m[strconv.Itoa(2000+i)] = "AddAccountKey"
	i++
	m[strconv.Itoa(2000+i)] = "AddEncodedAccountKey"
	i++
	m[strconv.Itoa(2000+i)] = "AllocateStorageIndex"
	i++
	m[strconv.Itoa(2000+i)] = "CreateAccount"
	i++
	m[strconv.Itoa(2000+i)] = "EmitEvent"
	i++
	m[strconv.Itoa(2000+i)] = "GenerateUUID"
	i++
	m[strconv.Itoa(2000+i)] = "GetAccountAvailableBalance"
	i++
	m[strconv.Itoa(2000+i)] = "GetAccountBalance"
	i++
	m[strconv.Itoa(2000+i)] = "GetAccountContractCode"
	i++
	m[strconv.Itoa(2000+i)] = "GetAccountContractNames"
	i++
	m[strconv.Itoa(2000+i)] = "GetAccountKey"
	i++
	m[strconv.Itoa(2000+i)] = "GetBlockAtHeight"
	i++
	m[strconv.Itoa(2000+i)] = "GetCode"
	i++
	m[strconv.Itoa(2000+i)] = "GetCurrentBlockHeight"
	i++
	m[strconv.Itoa(2000+i)] = "_"
	i++
	m[strconv.Itoa(2000+i)] = "GetStorageCapacity"
	i++
	m[strconv.Itoa(2000+i)] = "GetStorageUsed"
	i++
	m[strconv.Itoa(2000+i)] = "GetValue"
	i++
	m[strconv.Itoa(2000+i)] = "RemoveAccountContractCode"
	i++
	m[strconv.Itoa(2000+i)] = "ResolveLocation"
	i++
	m[strconv.Itoa(2000+i)] = "RevokeAccountKey"
	i++
	m[strconv.Itoa(2000+i)] = "RevokeEncodedAccountKey"
	i++
	m[strconv.Itoa(2000+i)] = "_"
	i++
	m[strconv.Itoa(2000+i)] = "SetValue"
	i++
	m[strconv.Itoa(2000+i)] = "UpdateAccountContractCode"
	i++
	m[strconv.Itoa(2000+i)] = "ValidatePublicKey"
	i++
	m[strconv.Itoa(2000+i)] = "ValueExists"
	i++
	m[strconv.Itoa(2000+i)] = "AccountKeysCount"
	i++
	m[strconv.Itoa(2000+i)] = "BLSVerifyPOP"
	i++
	m[strconv.Itoa(2000+i)] = "BLSAggregateSignatures"
	i++
	m[strconv.Itoa(2000+i)] = "BLSAggregatePublicKeys"
	i++
	m[strconv.Itoa(2000+i)] = "GetOrLoadProgram"
	i++
	m[strconv.Itoa(2000+i)] = "GenerateAccountLocalID"
	i++
	m[strconv.Itoa(2000+i)] = "GetRandomSourceHistory"
	i++
	m[strconv.Itoa(2000+i)] = "EVMGasUsage"
	i++
	m[strconv.Itoa(2000+i)] = "RLPEncoding"
	i++
	m[strconv.Itoa(2000+i)] = "RLPDecoding"
	i++
	m[strconv.Itoa(2000+i)] = "EncodeEvent"
	i++
	m[strconv.Itoa(2000+i)] = "_"
	i++
	m[strconv.Itoa(2000+i)] = "EVMEncodeABI"
	i++
	m[strconv.Itoa(2000+i)] = "EVMDecodeABI"

	return m
}()
