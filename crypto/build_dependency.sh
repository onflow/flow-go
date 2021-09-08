#!/bin/bash

set -euo pipefail

PKG_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
RELIC_DIR_NAME="relic"
RELIC_DIR="${PKG_DIR}/${RELIC_DIR_NAME}"

# grant permissions if not existant
if [[ ! -r ${PKG_DIR}  || ! -w ${PKG_DIR} || ! -x ${PKG_DIR} ]]; then
   sudo chmod -R 755 "${PKG_DIR}"
fi

rm -rf "${RELIC_DIR}"

# relic version or tag
relic_version="9a0128631841c7ade82460e8e80f8289cf9120b5"

# clone a specific version of Relic without history if it's tagged.
# git -c http.sslVerify=true clone --branch $(relic_version) --single-branch --depth 1 https://github.com/relic-toolkit/relic.git ${RELIC_DIR_NAME} || { echo "git clone failed"; exit 1; }

# clone all the history if the version is only defined by a commit hash.
git -c http.sslVerify=true clone --branch main --single-branch https://github.com/relic-toolkit/relic.git ${RELIC_DIR_NAME} || { echo "git clone failed"; exit 1; }

if [ -d "${RELIC_DIR}" ]
then
   (
      cd ${RELIC_DIR_NAME} || { echo "cd relic failed"; exit 1; }
      git checkout $relic_version
   )
   # build relic
   bash relic_build.sh
else 
   { echo "couldn't find relic directory"; exit 1; }
fi


