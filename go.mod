module github.com/onflow/flow-go

go 1.20

require (
	cloud.google.com/go/compute/metadata v0.2.3
	cloud.google.com/go/profiler v0.3.0
	cloud.google.com/go/storage v1.30.1
	github.com/antihax/optional v1.0.0
	github.com/aws/aws-sdk-go-v2/config v1.25.5
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.5.1
	github.com/aws/aws-sdk-go-v2/service/s3 v1.15.0
	github.com/btcsuite/btcd/btcec/v2 v2.2.1
	github.com/davecgh/go-spew v1.1.1
	github.com/dgraph-io/badger/v2 v2.2007.4
	github.com/ef-ds/deque v1.0.4
	github.com/ethereum/go-ethereum v1.13.5
	github.com/fxamacker/cbor/v2 v2.4.1-0.20230228173756-c0c9f774e40c
	github.com/gammazero/workerpool v1.1.2
	github.com/gogo/protobuf v1.3.2
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.3
	github.com/google/go-cmp v0.6.0
	github.com/google/pprof v0.0.0-20231229205709-960ae82b1e42
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/grpc-ecosystem/go-grpc-middleware/providers/zerolog/v2 v2.0.0-rc.2
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.0.0-rc.2
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.19.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/golang-lru v1.0.2
	github.com/improbable-eng/grpc-web v0.15.0
	github.com/ipfs/go-block-format v0.2.0
	github.com/ipfs/go-blockservice v0.4.0
	github.com/ipfs/go-cid v0.4.1
	github.com/ipfs/go-datastore v0.6.0
	github.com/ipfs/go-ds-badger2 v0.1.3
	github.com/ipfs/go-ipfs-blockstore v1.3.0
	github.com/ipfs/go-ipfs-provider v0.7.0
	github.com/ipfs/go-ipld-format v0.6.0
	github.com/ipfs/go-log v1.0.5
	github.com/ipfs/go-log/v2 v2.5.1
	github.com/libp2p/go-addr-util v0.1.0
	github.com/libp2p/go-libp2p v0.32.2
	github.com/libp2p/go-libp2p-kad-dht v0.25.2
	github.com/libp2p/go-libp2p-kbucket v0.6.3
	github.com/libp2p/go-libp2p-pubsub v0.10.0
	github.com/montanaflynn/stats v0.6.6
	github.com/multiformats/go-multiaddr v0.12.2
	github.com/multiformats/go-multiaddr-dns v0.3.1
	github.com/multiformats/go-multihash v0.2.3
	github.com/onflow/atree v0.6.0
	github.com/onflow/cadence v0.42.9
	github.com/onflow/crypto v0.25.0
	github.com/onflow/flow v0.3.4
	github.com/onflow/flow-core-contracts/lib/go/contracts v0.15.1
	github.com/onflow/flow-core-contracts/lib/go/templates v0.15.1
	github.com/onflow/flow-go-sdk v0.44.0
	github.com/onflow/flow/protobuf/go/flow v0.3.7
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58
	github.com/pierrec/lz4 v2.6.1+incompatible
	github.com/pkg/errors v0.9.1
	github.com/pkg/profile v1.7.0
	github.com/prometheus/client_golang v1.18.0
	github.com/rs/cors v1.8.0
	github.com/rs/zerolog v1.29.0
	github.com/schollz/progressbar/v3 v3.13.1
	github.com/sethvargo/go-retry v0.2.3
	github.com/shirou/gopsutil/v3 v3.22.2
	github.com/spf13/cobra v1.8.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.15.0
	github.com/stretchr/testify v1.8.4
	github.com/vmihailenco/msgpack v4.0.4+incompatible
	github.com/vmihailenco/msgpack/v4 v4.3.11
	go.opentelemetry.io/otel v1.22.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.21.0
	go.opentelemetry.io/otel/sdk v1.21.0
	go.opentelemetry.io/otel/trace v1.22.0
	go.uber.org/atomic v1.11.0
	go.uber.org/multierr v1.11.0
	golang.org/x/crypto v0.18.0
	golang.org/x/exp v0.0.0-20240119083558-1b970713d09a
	golang.org/x/sync v0.6.0
	golang.org/x/sys v0.16.0
	golang.org/x/text v0.14.0
	golang.org/x/time v0.3.0
	golang.org/x/tools v0.17.0
	google.golang.org/api v0.151.0
	google.golang.org/genproto v0.0.0-20240102182953-50ed04b92917
	google.golang.org/grpc v1.60.1
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.2.0
	google.golang.org/protobuf v1.32.0
	gotest.tools v2.2.0+incompatible
	pgregory.net/rapid v1.1.0
)

require (
	github.com/cockroachdb/pebble v0.0.0-20230928194634-aa077af62593
	github.com/coreos/go-semver v0.3.0
	github.com/docker/go-units v0.5.0
	github.com/go-playground/validator/v10 v10.14.1
	github.com/golang/snappy v0.0.5-0.20220116011046-fa5810519dcb
	github.com/gorilla/websocket v1.5.0
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/ipfs/boxo v0.17.0
	github.com/mitchellh/mapstructure v1.5.0
	github.com/onflow/flow-ft/lib/go/contracts v0.7.1-0.20230711213910-baad011d2b13
	github.com/onflow/flow-nft/lib/go/contracts v1.1.0
	github.com/onflow/wal v0.0.0-20240208022732-d756cd497d3b
	github.com/slok/go-http-metrics v0.10.0
	github.com/sony/gobreaker v0.5.0
	google.golang.org/genproto/googleapis/api v0.0.0-20240108191215-35c7eff3a6b1
	google.golang.org/genproto/googleapis/bytestream v0.0.0-20231030173426-d783a09b4405
	gopkg.in/yaml.v2 v2.4.0
)

require (
	cloud.google.com/go v0.111.0 // indirect
	cloud.google.com/go/compute v1.23.3 // indirect
	cloud.google.com/go/iam v1.1.5 // indirect
	github.com/DataDog/zstd v1.5.2 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/VictoriaMetrics/fastcache v1.12.1 // indirect
	github.com/aws/aws-sdk-go-v2 v1.23.1 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.16.4 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.14.5 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.2.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.7.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.10.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.10.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.7.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.17.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.20.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.25.4 // indirect
	github.com/aws/smithy-go v1.17.0 // indirect
	github.com/benbjohnson/clock v1.3.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.7.0 // indirect
	github.com/btcsuite/btcd/chaincfg/chainhash v1.0.2 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cockroachdb/errors v1.9.1 // indirect
	github.com/cockroachdb/logtags v0.0.0-20230118201751-21c54148d20b // indirect
	github.com/cockroachdb/redact v1.1.3 // indirect
	github.com/cockroachdb/tokenbucket v0.0.0-20230807174530-cc333fc44b06 // indirect
	github.com/consensys/bavard v0.1.13 // indirect
	github.com/consensys/gnark-crypto v0.12.1 // indirect
	github.com/containerd/cgroups v1.1.0 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/crate-crypto/go-kzg-4844 v0.7.0 // indirect
	github.com/cskr/pubsub v1.0.2 // indirect
	github.com/davidlazar/go-crypto v0.0.0-20200604182044-b73af7476f6c // indirect
	github.com/deckarep/golang-set/v2 v2.1.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.2.0 // indirect
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/dgraph-io/ristretto v0.1.0 // indirect
	github.com/dgryski/go-farm v0.0.0-20190423205320-6a90982ecee2 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/elastic/gosigar v0.14.2 // indirect
	github.com/ethereum/c-kzg-4844 v0.4.0 // indirect
	github.com/felixge/fgprof v0.9.3 // indirect
	github.com/flynn/noise v1.0.1 // indirect
	github.com/francoispqt/gojay v1.2.13 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/fxamacker/circlehash v0.3.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/gammazero/deque v0.1.0 // indirect
	github.com/getsentry/sentry-go v0.18.0 // indirect
	github.com/go-kit/kit v0.12.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gofrs/flock v0.8.1 // indirect
	github.com/golang/glog v1.1.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gax-go/v2 v2.12.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/holiman/bloomfilter/v2 v2.0.3 // indirect
	github.com/holiman/uint256 v1.2.3 // indirect
	github.com/huin/goupnp v1.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/ipfs/bbloom v0.0.4 // indirect
	github.com/ipfs/go-bitswap v0.9.0 // indirect
	github.com/ipfs/go-cidutil v0.1.0 // indirect
	github.com/ipfs/go-fetcher v1.5.0 // indirect
	github.com/ipfs/go-ipfs-delay v0.0.1 // indirect
	github.com/ipfs/go-ipfs-ds-help v1.1.0 // indirect
	github.com/ipfs/go-ipfs-exchange-interface v0.2.0 // indirect
	github.com/ipfs/go-ipfs-pq v0.0.3 // indirect
	github.com/ipfs/go-ipfs-util v0.0.3 // indirect
	github.com/ipfs/go-metrics-interface v0.0.1 // indirect
	github.com/ipfs/go-peertaskqueue v0.8.1 // indirect
	github.com/ipfs/go-verifcid v0.0.1 // indirect
	github.com/ipld/go-ipld-prime v0.21.0 // indirect
	github.com/jackpal/go-nat-pmp v1.0.2 // indirect
	github.com/jbenet/go-temp-err-catcher v0.1.0 // indirect
	github.com/jbenet/goprocess v0.1.4 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/k0kubun/pp/v3 v3.2.0 // indirect
	github.com/kevinburke/go-bindata v3.23.0+incompatible // indirect
	github.com/klauspost/compress v1.17.4 // indirect
	github.com/klauspost/cpuid/v2 v2.2.6 // indirect
	github.com/koron/go-ssdp v0.0.4 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/libp2p/go-buffer-pool v0.1.0 // indirect
	github.com/libp2p/go-cidranger v1.1.0 // indirect
	github.com/libp2p/go-flow-metrics v0.1.0 // indirect
	github.com/libp2p/go-libp2p-asn-util v0.4.1 // indirect
	github.com/libp2p/go-libp2p-core v0.20.1 // indirect
	github.com/libp2p/go-libp2p-record v0.2.0 // indirect
	github.com/libp2p/go-libp2p-routing-helpers v0.7.3 // indirect
	github.com/libp2p/go-msgio v0.3.0 // indirect
	github.com/libp2p/go-nat v0.2.0 // indirect
	github.com/libp2p/go-netroute v0.2.1 // indirect
	github.com/libp2p/go-reuseport v0.4.0 // indirect
	github.com/libp2p/go-yamux/v4 v4.0.1 // indirect
	github.com/logrusorgru/aurora/v4 v4.0.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/marten-seemann/tcp v0.0.0-20210406111302-dfbc87cc63fd // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/miekg/dns v1.1.57 // indirect
	github.com/mikioh/tcpinfo v0.0.0-20190314235526-30a79bb1804b // indirect
	github.com/mikioh/tcpopt v0.0.0-20190314235656-172688c1accc // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/mmcloughlin/addchain v0.4.0 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/multiformats/go-base32 v0.1.0 // indirect
	github.com/multiformats/go-base36 v0.2.0 // indirect
	github.com/multiformats/go-multiaddr-fmt v0.1.0 // indirect
	github.com/multiformats/go-multibase v0.2.0 // indirect
	github.com/multiformats/go-multicodec v0.9.0 // indirect
	github.com/multiformats/go-multistream v0.5.0 // indirect
	github.com/multiformats/go-varint v0.0.7 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/olekukonko/tablewriter v0.0.5 // indirect
	github.com/onflow/sdks v0.5.0 // indirect
	github.com/onsi/ginkgo/v2 v2.13.2 // indirect
	github.com/opencontainers/runtime-spec v1.1.0 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pelletier/go-toml/v2 v2.0.6 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/polydawn/refmt v0.89.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.45.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/psiemens/sconfig v0.1.0 // indirect
	github.com/quic-go/qpack v0.4.0 // indirect
	github.com/quic-go/qtls-go1-20 v0.4.1 // indirect
	github.com/quic-go/quic-go v0.40.1 // indirect
	github.com/quic-go/webtransport-go v0.6.0 // indirect
	github.com/raulk/go-watchdog v1.3.0 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	github.com/shirou/gopsutil v3.21.4-0.20210419000835-c7a38de76ee5+incompatible // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/afero v1.10.0 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/subosito/gotenv v1.4.2 // indirect
	github.com/supranational/blst v0.3.11 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20210819022825-2ae1ddf74ef7 // indirect
	github.com/texttheater/golang-levenshtein/levenshtein v0.0.0-20200805054039-cae8b0eaed6c // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/turbolent/prettier v0.0.0-20220320183459-661cc755135d // indirect
	github.com/vmihailenco/tagparser v0.1.1 // indirect
	github.com/whyrusleeping/go-keyspace v0.0.0-20160322163242-5b898ac5add1 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	github.com/zeebo/blake3 v0.2.3 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.21.0 // indirect
	go.opentelemetry.io/otel/metric v1.22.0 // indirect
	go.opentelemetry.io/proto/otlp v1.0.0 // indirect
	go.uber.org/dig v1.17.1 // indirect
	go.uber.org/fx v1.20.1 // indirect
	go.uber.org/mock v0.4.0 // indirect
	go.uber.org/zap v1.26.0 // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/net v0.20.0 // indirect
	golang.org/x/oauth2 v0.16.0 // indirect
	golang.org/x/term v0.16.0 // indirect
	golang.org/x/xerrors v0.0.0-20231012003039-104605ab7028 // indirect
	gonum.org/v1/gonum v0.14.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240108191215-35c7eff3a6b1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/blake3 v1.2.1 // indirect
	nhooyr.io/websocket v1.8.7 // indirect
	rsc.io/tmplfunc v0.0.3 // indirect
)

// Using custom fork until https://github.com/onflow/flow-go/issues/5338 is resolved
replace github.com/ipfs/boxo => github.com/onflow/boxo v0.0.0-20240201202436-f2477b92f483

replace github.com/ethereum/go-ethereum v1.13.5 => github.com/onflow/go-ethereum v0.0.2
