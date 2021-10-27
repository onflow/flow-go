module github.com/onflow/flow-go

go 1.15

require (
	cloud.google.com/go/storage v1.16.0
	github.com/aws/aws-sdk-go-v2/config v1.8.0
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.5.1
	github.com/aws/aws-sdk-go-v2/service/s3 v1.15.0
	github.com/bsipos/thist v1.0.0
	github.com/btcsuite/btcd v0.21.0-beta
	github.com/dapperlabs/testingdock v0.4.2
	github.com/davecgh/go-spew v1.1.1
	github.com/dgraph-io/badger/v2 v2.0.3
	github.com/ef-ds/deque v1.0.4
	github.com/ethereum/go-ethereum v1.9.13
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/fxamacker/cbor/v2 v2.2.1-0.20210927235116-3d6d5d1de29b
	github.com/gammazero/workerpool v1.1.2
	github.com/gogo/protobuf v1.3.2
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.6
	github.com/google/uuid v1.3.0
	github.com/grpc-ecosystem/go-grpc-middleware/providers/zerolog/v2 v2.0.0-rc.2
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.0.0-20200501113911-9a95f0fdbfea
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.6.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/golang-lru v0.5.4
	github.com/improbable-eng/grpc-web v0.12.0
	github.com/ipfs/go-log v1.0.5
	github.com/jrick/bitset v1.0.0
	github.com/kr/text v0.2.0 // indirect
	github.com/libp2p/go-addr-util v0.1.0
	github.com/libp2p/go-libp2p v0.14.4
	github.com/libp2p/go-libp2p-core v0.8.6
	github.com/libp2p/go-libp2p-discovery v0.5.0
	github.com/libp2p/go-libp2p-kad-dht v0.13.0
	github.com/libp2p/go-libp2p-pubsub v0.4.1
	github.com/libp2p/go-libp2p-swarm v0.5.3
	github.com/libp2p/go-libp2p-tls v0.1.3
	github.com/libp2p/go-libp2p-transport-upgrader v0.4.6
	github.com/libp2p/go-tcp-transport v0.2.7
	github.com/m4ksio/wal v1.0.0
	github.com/multiformats/go-multiaddr v0.3.3
	github.com/multiformats/go-multiaddr-dns v0.3.1
	github.com/onflow/atree v0.1.0-beta1
	github.com/onflow/cadence v0.19.1-0.20210920215340-75c54c6c01eb
	github.com/onflow/flow-core-contracts/lib/go/contracts v0.7.9
	github.com/onflow/flow-core-contracts/lib/go/templates v0.7.9
	github.com/onflow/flow-emulator v0.20.3
	github.com/onflow/flow-go-sdk v0.21.0
	github.com/onflow/flow-go/crypto v0.21.3
	github.com/onflow/flow/protobuf/go/flow v0.2.2
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.10.0
	github.com/psiemens/sconfig v0.1.0 // indirect
	github.com/rs/zerolog v1.19.0
	github.com/schollz/progressbar/v3 v3.7.6
	github.com/sethvargo/go-retry v0.1.0
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/uber/jaeger-client-go v2.29.1+incompatible
	github.com/uber/jaeger-lib v2.4.0+incompatible // indirect
	github.com/vmihailenco/msgpack v4.0.4+incompatible
	github.com/vmihailenco/msgpack/v4 v4.3.11
	go.uber.org/atomic v1.7.0
	go.uber.org/multierr v1.6.0
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
	golang.org/x/exp v0.0.0-20200224162631-6cc2880d07d6
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210910150752-751e447fb3d0 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	google.golang.org/api v0.49.0
	google.golang.org/genproto v0.0.0-20210903162649-d08c68adba83
	google.golang.org/grpc v1.40.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.1.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/ini.v1 v1.63.0 // indirect
	gotest.tools v2.2.0+incompatible
	pgregory.net/rapid v0.4.7
)

replace mellium.im/sasl => github.com/mellium/sasl v0.2.1

replace github.com/onflow/flow-go/crypto => ./crypto
