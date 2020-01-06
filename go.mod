module github.com/dapperlabs/flow-go

go 1.13

require (
	github.com/antlr/antlr4 v0.0.0-20190723154043-128983ff903e
	github.com/c-bata/go-prompt v0.2.3
	github.com/dchest/siphash v1.2.1
	github.com/dgraph-io/badger/v2 v2.0.0
	github.com/ethereum/go-ethereum v1.9.9
	github.com/go-test/deep v1.0.4
	github.com/gogo/protobuf v1.3.1
	github.com/golang/mock v1.3.1
	github.com/golang/protobuf v1.3.2
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/golang-lru v0.5.3
	github.com/ipfs/go-log v0.0.1
	github.com/jrick/bitset v1.0.0
	github.com/juju/loggo v0.0.0-20190526231331-6e530bcce5d8
	github.com/libp2p/go-libp2p v0.4.2
	github.com/libp2p/go-libp2p-core v0.2.5
	github.com/libp2p/go-libp2p-pubsub v0.2.5
	github.com/libp2p/go-tcp-transport v0.1.1
	github.com/logrusorgru/aurora v0.0.0-20191116043053-66b7ad493a23
	github.com/magiconair/properties v1.8.1
	github.com/mattn/go-tty v0.0.3 // indirect
	github.com/multiformats/go-multiaddr v0.2.0
	github.com/onsi/gomega v1.8.1 // indirect
	github.com/philhofer/fwd v1.0.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/pkg/term v0.0.0-20190109203006-aa71e9d9e942 // indirect
	github.com/raviqqe/hamt v0.0.0-20190615202029-864fb7caef85
	github.com/rivo/uniseg v0.1.0
	github.com/rs/zerolog v1.15.0
	github.com/segmentio/fasthash v1.0.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.4.0
	github.com/tinylib/msgp v1.1.1 // indirect
	github.com/whyrusleeping/go-logging v0.0.0-20170515211332-0457bb6b88fc
	go.uber.org/atomic v1.4.0
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550
	golang.org/x/sys v0.0.0-20191220142924-d4481acd189f // indirect
	golang.org/x/text v0.3.2
	golang.org/x/tools v0.0.0-20190524140312-2c0ae7006135 // indirect
	gonum.org/v1/gonum v0.0.0-20191018104224-74cb7b153f2c
	google.golang.org/grpc v1.26.0
	honnef.co/go/tools v0.0.0-20190523083050-ea95bdfd59fc // indirect
	zombiezen.com/go/capnproto2 v2.17.0+incompatible
)

replace mellium.im/sasl => github.com/mellium/sasl v0.2.1
