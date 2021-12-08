
#!/bin/bash

# crypto package 
PKG_NAME="github.com/onflow/flow-go/crypto"

# go get the package
go get ${PKG_NAME}

# go.mod
MOD_FILE="./go.mod"

# the version of onflow/flow-go/crypto used in the project is read from the go.mod file
if [ -f "${MOD_FILE}" ]
then
    # extract the version from the go.mod file
    VERSION="$(grep ${PKG_NAME} < ${MOD_FILE} | cut -d' ' -f 2)"
    # using the right version, get the package directory path
    PKG_DIR="$(go env GOPATH)/pkg/mod/${PKG_NAME}@${VERSION}"
else 
   { echo "couldn't find go.mod file - make sure the script is in the project root directory"; exit 1; }
fi

# grant permissions if not existant
if [[ ! -r ${PKG_DIR}  || ! -w ${PKG_DIR} || ! -x ${PKG_DIR} ]]; then
   sudo chmod -R 755 "${PKG_DIR}"
fi

# get into the package directory and set up the external dependencies
(
    cd "${PKG_DIR}" || { echo "cd into the GOPATH package folder failed"; exit 1; }
    go generate
)