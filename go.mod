module github.com/onflow/flow-go

go 1.16

require (
	cloud.google.com/go/storage v1.16.0
	github.com/antihax/optional v1.0.0
	github.com/aws/aws-sdk-go-v2/config v1.8.0
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.5.1
	github.com/aws/aws-sdk-go-v2/service/s3 v1.15.0
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/bsipos/thist v1.0.0
	github.com/btcsuite/btcd v0.22.0-beta
	github.com/dapperlabs/testingdock v0.4.4
	github.com/davecgh/go-spew v1.1.1
	github.com/dgraph-io/badger/v2 v2.2007.3
	github.com/ef-ds/deque v1.0.4
	github.com/ethereum/go-ethereum v1.9.13
	github.com/fxamacker/cbor/v2 v2.3.1-0.20211029162100-5d5d7c3edd41
	github.com/gammazero/workerpool v1.1.2
	github.com/gogo/protobuf v1.3.2
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.7
	github.com/google/uuid v1.3.0
	github.com/gopherjs/gopherjs v0.0.0-20190430165422-3e4dfb77656c // indirect
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/go-grpc-middleware/providers/zerolog/v2 v2.0.0-rc.2
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.0.0-20200501113911-9a95f0fdbfea
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.6.0
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/golang-lru v0.5.4
	github.com/improbable-eng/grpc-web v0.12.0
	github.com/ipfs/go-bitswap v0.5.0
	github.com/ipfs/go-block-format v0.0.3
	github.com/ipfs/go-blockservice v0.2.0
	github.com/ipfs/go-cid v0.1.0
	github.com/ipfs/go-datastore v0.5.1
	github.com/ipfs/go-ds-badger2 v0.1.3
	github.com/ipfs/go-ipfs-blockstore v0.2.0
	github.com/ipfs/go-ipfs-provider v0.7.0
	github.com/ipfs/go-log v1.0.5
	github.com/ipld/go-ipld-prime v0.14.1 // indirect
	github.com/libp2p/go-addr-util v0.1.0
	github.com/libp2p/go-libp2p v0.16.0
	github.com/libp2p/go-libp2p-core v0.11.0
	github.com/libp2p/go-libp2p-discovery v0.6.0
	github.com/libp2p/go-libp2p-kad-dht v0.15.0
	github.com/libp2p/go-libp2p-pubsub v0.6.0
	github.com/libp2p/go-libp2p-swarm v0.8.0
	github.com/libp2p/go-libp2p-tls v0.3.1
	github.com/libp2p/go-libp2p-transport-upgrader v0.5.0
	github.com/libp2p/go-tcp-transport v0.4.0
	github.com/m4ksio/wal v1.0.0
	github.com/multiformats/go-multiaddr v0.4.1
	github.com/multiformats/go-multiaddr-dns v0.3.1
	github.com/multiformats/go-multihash v0.1.0
	github.com/onflow/atree v0.2.0
	github.com/onflow/cadence v0.23.2
	github.com/onflow/flow v0.2.3-0.20220131193101-d4e2ca43a621
	github.com/onflow/flow-core-contracts/lib/go/contracts v0.10.2-0.20220303222312-6b9de4f3fea5
	github.com/onflow/flow-core-contracts/lib/go/templates v0.10.1
	github.com/onflow/flow-emulator v0.20.3
	github.com/onflow/flow-go-sdk v0.24.0
	github.com/onflow/flow-go/crypto v0.24.3
	github.com/onflow/flow/protobuf/go/flow v0.2.4-0.20220304041411-6d91cd04a33a
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pierrec/lz4 v2.6.1+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/psiemens/sconfig v0.1.0 // indirect
	github.com/rivo/uniseg v0.2.1-0.20211004051800-57c86be7915a // indirect
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	github.com/rs/cors v1.8.0
	github.com/rs/zerolog v1.19.0
	github.com/schollz/progressbar/v3 v3.8.3
	github.com/sethvargo/go-retry v0.1.0
	github.com/shirou/gopsutil/v3 v3.22.2
	github.com/smartystreets/assertions v1.0.0 // indirect
	github.com/spf13/afero v1.8.0 // indirect
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.10.1
	github.com/stretchr/testify v1.7.1-0.20210824115523-ab6dc3262822
	github.com/uber/jaeger-client-go v2.29.1+incompatible
	github.com/uber/jaeger-lib v2.4.0+incompatible // indirect
	github.com/vmihailenco/msgpack v4.0.4+incompatible
	github.com/vmihailenco/msgpack/v4 v4.3.11
	go.uber.org/atomic v1.9.0
	go.uber.org/multierr v1.7.0
	golang.org/x/crypto v0.0.0-20211108221036-ceb1ce70b4fa
	golang.org/x/exp v0.0.0-20200224162631-6cc2880d07d6
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20220209214540-3681064d5158 // indirect
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba
	google.golang.org/api v0.63.0
	google.golang.org/genproto v0.0.0-20220211171837-173942840c17
	google.golang.org/grpc v1.44.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.1.0
	google.golang.org/protobuf v1.27.1
	gotest.tools v2.2.0+incompatible
	pgregory.net/rapid v0.4.7
)

replace mellium.im/sasl => github.com/mellium/sasl v0.2.1
