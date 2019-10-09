module github.com/dapperlabs/flow-go

go 1.12

require (
	github.com/antlr/antlr4 v0.0.0-20190723154043-128983ff903e
	github.com/bluele/gcache v0.0.0-20190518031135-bc40bd653833
	github.com/ethereum/go-ethereum v1.9.2
	github.com/go-pg/migrations v6.7.3+incompatible
	github.com/go-pg/pg v8.0.4+incompatible
	github.com/gogo/protobuf v1.2.1
	github.com/golang/mock v1.3.1
	github.com/golang/protobuf v1.3.2
	github.com/google/uuid v1.1.1
	github.com/google/wire v0.3.0
	github.com/improbable-eng/grpc-web v0.11.0
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/logrusorgru/aurora v0.0.0-20190428105938-cea283e61946
	github.com/myesui/uuid v1.0.0 // indirect
	github.com/onsi/ginkgo v1.8.0 // indirect
	github.com/onsi/gomega v1.5.0
	github.com/psiemens/sconfig v0.0.0-20190623041652-6e01eb1354fc
	github.com/raviqqe/hamt v0.0.0-20190615202029-864fb7caef85
	github.com/rivo/uniseg v0.1.0
	github.com/rs/cors v1.7.0 // indirect
	github.com/rs/zerolog v1.15.0
	github.com/segmentio/fasthash v1.0.0
	github.com/sirupsen/logrus v1.4.2
	github.com/sourcegraph/jsonrpc2 v0.0.0-20190106185902-35a74f039c6a
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/testify v1.3.0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4
	golang.org/x/net v0.0.0-20190628185345-da137c7871d7 // indirect
	golang.org/x/sys v0.0.0-20190626221950-04f50cda93cb // indirect
	golang.org/x/text v0.3.2
	google.golang.org/grpc v1.21.1
	gopkg.in/myesui/uuid.v1 v1.0.0
	gotest.tools v2.2.0+incompatible // indirect
	mellium.im/sasl v0.2.1 // indirect
)

replace mellium.im/sasl => github.com/mellium/sasl v0.2.1
