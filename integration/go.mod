module github.com/onflow/flow-go/integration

go 1.13

require (
	github.com/HdrHistogram/hdrhistogram-go v1.1.0 // indirect
	github.com/dapperlabs/testingdock v0.4.3-0.20200626075145-ea23fc16bb90
	github.com/dgraph-io/badger/v2 v2.0.3
	github.com/docker/docker v1.4.2-0.20190513124817-8c8457b0f2f8
	github.com/docker/go-connections v0.4.0
	github.com/go-openapi/strfmt v0.19.5 // indirect
	github.com/jedib0t/go-pretty v4.3.0+incompatible
	github.com/onflow/cadence v0.14.1
	github.com/onflow/flow-go v0.14.1-0.20210313151753-5612cf6db7ad // replaced by version on-disk
	github.com/onflow/flow-go-sdk v0.15.0
	github.com/onflow/flow-go/crypto v0.12.0 // replaced by version on-disk
	github.com/onflow/flow/protobuf/go/flow v0.1.9
	github.com/plus3it/gorecurcopy v0.0.1
	github.com/rs/zerolog v1.19.0
	github.com/stretchr/testify v1.7.0
	github.com/uber/jaeger-lib v2.4.0+incompatible // indirect
	google.golang.org/grpc v1.31.1
	gopkg.in/yaml.v2 v2.3.0
)

// temp fix for MacOS build. See comment https://github.com/ory/dockertest/issues/208#issuecomment-686820414
replace golang.org/x/sys => golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6

replace github.com/onflow/flow-go => ../

replace github.com/onflow/flow-go/crypto => ../crypto
