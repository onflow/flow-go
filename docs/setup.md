# Project Setup

## Environment setup

### Go
- Download and install Go 1.12 https://golang.org/dl/
- Create your worksapce `$GOPATH` directory and update your bash_profile file: export `$GOPATH=$HOME/directory/`. It is also suggested to update the path `$PATH` to use third party GO binaries: `$export PATH="$PATH:$GOPATH/bin" `
- Test a first go program https://golang.org/doc/install#testing
- Clone in this repo under your `$GOPATH`: `$GOPATH/src/github.com/dapperlabs/bamboo-node/`. Technically since we are using go modules and we prepend every `go` command with `GO111MODULE=on`, you can also try to clone this repo anywhere you want.

### Docker
- Download and install Docker https://docs.docker.com/docker-for-mac/install/
- Test docker by running integration tests `$ ./test.sh`. First time run should take a while because some base layers will be downloaded and built for the first time. See https://github.com/dapperlabs/bamboo-node#test for more details.

## Build (updates go modules)
```
$ GO111MODULE=on go build -o donotcommit ./cmd/execute/
$ GO111MODULE=on go build -o donotcommit ./cmd/security/
$ GO111MODULE=on go build -o donotcommit ./cmd/testhelpers/
```
TODO: move to Makefile


## Generate dependency injection
### Prerequisite 
Install wire: `$ GO111MODULE=on go get -u github.com/google/wire/cmd/wire`
### Command
```
$ GO111MODULE=on wire ./internal/execute/
$ GO111MODULE=on wire ./internal/security/
$ GO111MODULE=on wire ./internal/access/
```
TODO: move to Makefile

## Generate protobufs 
### Prerequisite 
1. Install prototools https://github.com/uber/prototool#installation  
2. $ go get -u github.com/golang/protobuf/protoc-gen-go
### Command
```
$ prototool generate proto/
```
TODO: move to Makefile
