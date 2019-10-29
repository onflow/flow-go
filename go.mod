module github.com/dapperlabs/flow-go

go 1.12

require (
	github.com/OneOfOne/xxhash v1.2.5 // indirect
	github.com/antlr/antlr4 v0.0.0-20190723154043-128983ff903e
	github.com/bluele/gcache v0.0.0-20190518031135-bc40bd653833
	github.com/c-bata/go-prompt v0.2.3
	github.com/davecgh/go-xdr v0.0.0-20161123171359-e6a2ba005892
	github.com/dchest/siphash v1.2.1
	github.com/dgraph-io/ristretto v0.0.0-20191010170704-2ba187ef9534
	github.com/ethereum/go-ethereum v1.9.2
	github.com/go-pg/migrations v6.7.3+incompatible
	github.com/go-pg/pg v8.0.4+incompatible
	github.com/gogo/protobuf v1.2.1
	github.com/golang/mock v1.3.1
	github.com/golang/protobuf v1.3.2
	github.com/google/wire v0.3.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/improbable-eng/grpc-web v0.11.0
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/kelindar/binary v1.0.7
	github.com/kr/pty v1.1.3 // indirect
	github.com/lib/pq v1.1.1 // indirect
	github.com/logrusorgru/aurora v0.0.0-20190428105938-cea283e61946
	github.com/mattn/go-runewidth v0.0.5 // indirect
	github.com/onsi/ginkgo v1.8.0 // indirect
	github.com/onsi/gomega v1.5.0
	github.com/pkg/errors v0.8.1
	github.com/pkg/term v0.0.0-20190109203006-aa71e9d9e942 // indirect
	github.com/psiemens/sconfig v0.0.0-20190623041652-6e01eb1354fc
	github.com/raviqqe/hamt v0.0.0-20190615202029-864fb7caef85
	github.com/rivo/uniseg v0.1.0
	github.com/rs/cors v1.7.0 // indirect
	github.com/rs/zerolog v1.15.0
	github.com/segmentio/fasthash v1.0.0
	github.com/sirupsen/logrus v1.4.2
	github.com/sourcegraph/jsonrpc2 v0.0.0-20190106185902-35a74f039c6a
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.4.0
	github.com/twinj/uuid v1.0.0
	github.com/urfave/cli v1.22.1
	github.com/urfave/cli/v2 v2.0.0-alpha.2
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4
	golang.org/x/lint v0.0.0-20190930215403-16217165b5de // indirect
	golang.org/x/net v0.0.0-20190628185345-da137c7871d7 // indirect
	golang.org/x/text v0.3.2
	gonum.org/v1/gonum v0.0.0-20191018104224-74cb7b153f2c
	google.golang.org/grpc v1.21.1
	mellium.im/sasl v0.2.1 // indirect
	zombiezen.com/go/capnproto2 v2.17.0+incompatible
)

replace mellium.im/sasl => github.com/mellium/sasl v0.2.1
