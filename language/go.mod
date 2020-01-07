module github.com/dapperlabs/flow-go/language

go 1.13

require (
	github.com/antlr/antlr4 v0.0.0-20191217191749-ff67971f8580
	github.com/c-bata/go-prompt v0.2.3
	github.com/dapperlabs/flow-go v0.0.0-00010101000000-000000000000
	github.com/logrusorgru/aurora v0.0.0-20191116043053-66b7ad493a23
	github.com/nsf/jsondiff v0.0.0-20190712045011-8443391ee9b6
	github.com/raviqqe/hamt v0.0.0-20190615202029-864fb7caef85
	github.com/rivo/uniseg v0.1.0
	github.com/segmentio/fasthash v1.0.1
	github.com/sourcegraph/jsonrpc2 v0.0.0-20191222043438-96c4efab7ee2
	github.com/stretchr/testify v1.4.0
	github.com/xeipuuv/gojsonschema v1.2.0
	golang.org/x/text v0.3.2
	google.golang.org/grpc v1.26.0
)

replace github.com/dapperlabs/flow-go => ../

replace github.com/dapperlabs/flow-go/language => ./

replace github.com/dapperlabs/flow-go/crypto => ../crypto

replace github.com/dapperlabs/flow-go/protobuf => ../protobuf
