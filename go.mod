module github.com/onflow/flow-go

go 1.13

require (
	cloud.google.com/go/storage v1.10.0
	github.com/HdrHistogram/hdrhistogram-go v0.9.0 // indirect
	github.com/bsipos/thist v1.0.0
	github.com/btcsuite/btcd v0.21.0-beta
	github.com/codahale/hdrhistogram v0.9.0 // indirect
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/dgraph-io/badger/v2 v2.0.3
	github.com/ef-ds/deque v1.0.4
	github.com/ethereum/go-ethereum v1.9.13
	github.com/fxamacker/cbor/v2 v2.2.1-0.20210510192846-c3f3c69e7bc8
	github.com/gogo/protobuf v1.3.2
	github.com/golang/mock v1.5.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.5
	github.com/google/uuid v1.1.2
	github.com/grpc-ecosystem/go-grpc-middleware/providers/zerolog/v2 v2.0.0-rc.2
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.0.0-20200501113911-9a95f0fdbfea
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/improbable-eng/grpc-web v0.12.0
	github.com/ipfs/go-log v1.0.4
	github.com/jrick/bitset v1.0.0
	github.com/libp2p/go-addr-util v0.0.2
	github.com/libp2p/go-libp2p v0.14.1
	github.com/libp2p/go-libp2p-core v0.8.5
	github.com/libp2p/go-libp2p-discovery v0.5.0
	github.com/libp2p/go-libp2p-pubsub v0.4.1
	github.com/libp2p/go-libp2p-swarm v0.5.0
	github.com/libp2p/go-libp2p-transport-upgrader v0.4.2
	github.com/libp2p/go-tcp-transport v0.2.1
	github.com/m4ksio/wal v1.0.0
	github.com/marten-seemann/qtls-go1-15 v0.1.4 // indirect
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/onflow/cadence v0.17.1-0.20210610211746-3141dd5d2e38
	github.com/onflow/flow-core-contracts/lib/go/contracts v0.7.3-0.20210527134022-58c25247091a
	github.com/onflow/flow-go-sdk v0.20.0-alpha.1
	github.com/onflow/flow-go/crypto v0.18.0
	github.com/onflow/flow/protobuf/go/flow v0.2.0
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/rs/zerolog v1.19.0
	github.com/spf13/cobra v0.0.6
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.4.0
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/uber/jaeger-client-go v2.22.1+incompatible
	github.com/uber/jaeger-lib v2.3.0+incompatible // indirect
	github.com/vmihailenco/msgpack v4.0.4+incompatible
	github.com/vmihailenco/msgpack/v4 v4.3.11
	go.uber.org/atomic v1.7.0
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
	golang.org/x/exp v0.0.0-20200224162631-6cc2880d07d6
	golang.org/x/sys v0.0.0-20210603125802-9665404d3644 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	google.golang.org/api v0.31.0
	google.golang.org/grpc v1.33.2
	google.golang.org/protobuf v1.26.0
	gotest.tools v2.2.0+incompatible
)

replace mellium.im/sasl => github.com/mellium/sasl v0.2.1

replace github.com/onflow/flow-go/crypto => ./crypto
