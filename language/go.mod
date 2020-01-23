module github.com/dapperlabs/flow-go/language

go 1.13

require (
	github.com/antlr/antlr4 v0.0.0-20191217191749-ff67971f8580
	github.com/c-bata/go-prompt v0.2.3
	github.com/dapperlabs/flow-go v0.2.5-beta
	github.com/logrusorgru/aurora v0.0.0-20191116043053-66b7ad493a23
	github.com/raviqqe/hamt v0.0.0-20190615202029-864fb7caef85
	github.com/rivo/uniseg v0.1.0
	github.com/segmentio/fasthash v1.0.1
	github.com/stretchr/testify v1.4.0
	golang.org/x/text v0.3.2
)

replace github.com/dapperlabs/flow-go => ../

replace github.com/dapperlabs/flow-go/language => ./

replace github.com/dapperlabs/flow-go/crypto => ../crypto

replace github.com/dapperlabs/flow-go/protobuf => ../protobuf
