module github.com/dapperlabs/flow-go

go 1.13

replace mellium.im/sasl => github.com/mellium/sasl v0.2.1

replace github.com/dapperlabs/flow-go => ./

replace github.com/dapperlabs/flow-go/crypto => ./crypto

replace github.com/dapperlabs/flow-go/protobuf => ./protobuf

replace github.com/dapperlabs/flow-go/integration => ./integration

require (
	github.com/dapperlabs/flow-go/crypto v0.0.0-00010101000000-000000000000
	github.com/dgraph-io/badger/v2 v2.0.3
	github.com/ethereum/go-ethereum v1.9.15
	github.com/jrick/bitset v1.0.0
	github.com/opentracing/opentracing-go v1.1.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.6.0
	github.com/rs/zerolog v1.19.0
	github.com/stretchr/testify v1.4.0
	github.com/uber/jaeger-client-go v2.24.0+incompatible
	github.com/uber/jaeger-lib v2.2.0+incompatible // indirect
	github.com/vmihailenco/msgpack v4.0.4+incompatible
	github.com/vmihailenco/msgpack/v4 v4.3.12
	go.uber.org/atomic v1.6.0 // indirect
	golang.org/x/crypto v0.0.0-20200311171314-f7b00557c8c4
)
