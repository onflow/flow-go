# Instructions
These instructions are intended to walk you through how to set up an example sandbox to test out and play with the BPL.

## Installation

### Install Go
- Download and install [Go 1.12](https://golang.org/doc/install)
- Create your workspace `$GOPATH` directory and update your bash_profile to contain the following:

```bash
export `$GOPATH=$HOME/path-to-your-go-workspace/`
```

It's also a good idea to update your `$PATH` to use third party GO binaries: 

```bash
export PATH="$PATH:$GOPATH/bin"
```

- Test that Go was installed correctly: https://golang.org/doc/install#testing


### Set up the Repo

***Clone the bamboo-node repo***

```bash
git clone git@github.com:dapperlabs/bamboo-node.git
```


***Install the CLI tool***

You may need to install 

```bash
cd bamboo-node

rm -r pkg/crypto/relic
git submodule update --init --recursive
brew install cmake

make install-cli

# Test the command
bamboo --help
```