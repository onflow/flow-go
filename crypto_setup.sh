
#!/bin/bash

# go get the package
go get github.com/onflow/flow-go/crypto

# the version of flow-go/crypto used for now is v0.23.3
# till the script is automatized, the version is hardcoded.
VERSION="v0.23.3"
PKG_DIR="${GOPATH}/pkg/mod/github.com/onflow/flow-go/crypto@${VERSION}"

# grant permissions if not existant
if [[ ! -r ${PKG_DIR}  || ! -w ${PKG_DIR} || ! -x ${PKG_DIR} ]]; then
   sudo chmod -R 755 "${PKG_DIR}"
fi

(
    cd "${PKG_DIR}" || { echo "cd into the GOPATH package folder failed"; exit 1; }

    go generate
)