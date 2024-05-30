package debug

import (
	"encoding/json"
	"fmt"

	"github.com/onflow/go-ethereum/eth/tracers"
)

type TransactionTracer struct {
	tracer tracers.Tracer
}

func NewTransactionTracer() (*TransactionTracer, error) {
	tracerConf := json.RawMessage{}
	if err := tracerConf.UnmarshalJSON([]byte(`{ "onlyTopCall": true }`)); err != nil {
		return nil, err
	}

	tracer, err := tracers.DefaultDirectory.New("callTracer", &tracers.Context{}, tracerConf)
	if err != nil {
		return nil, err
	}

	return &TransactionTracer{
		tracer,
	}, nil
}

func (t *TransactionTracer) Tracer() tracers.Tracer {
	return t.tracer
}

func (t *TransactionTracer) Upload() {
	res, _ := t.tracer.GetResult()
	fmt.Println(res)
}
