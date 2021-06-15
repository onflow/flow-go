module github.com/onflow/flow-go/integration

go 1.15

require (
	github.com/dapperlabs/testingdock v0.4.3-0.20200626075145-ea23fc16bb90
	github.com/dgraph-io/badger/v2 v2.0.3
	github.com/docker/docker v1.4.2-0.20190513124817-8c8457b0f2f8
	github.com/docker/go-connections v0.4.0
	github.com/go-openapi/strfmt v0.20.0 // indirect
	github.com/jedib0t/go-pretty v4.3.0+incompatible
	github.com/onflow/cadence v0.17.1-0.20210610211746-3141dd5d2e38
	github.com/onflow/flow-go v0.11.1 // replaced by version on-disk
	github.com/onflow/flow-go-sdk v0.20.0-alpha.1
	github.com/onflow/flow-go/crypto v0.18.0 // replaced by version on-disk
	github.com/onflow/flow/protobuf/go/flow v0.2.0
	github.com/plus3it/gorecurcopy v0.0.1
	github.com/rs/zerolog v1.19.0
	github.com/stretchr/testify v1.7.0
	google.golang.org/grpc v1.33.2
	gopkg.in/yaml.v2 v2.3.0
)

// temp fix for MacOS build. See comment https://github.com/ory/dockertest/issues/208#issuecomment-686820414
replace golang.org/x/sys => golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6

replace github.com/onflow/flow-go => ../

replace github.com/onflow/flow-go/crypto => ../crypto
