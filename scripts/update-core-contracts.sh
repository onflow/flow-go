#!/bin/sh
#
# This script updates all flow-core-contracts dependencies to a new version.
# Specify the desired version as the only argument when running the script:
#   ./scripts/update-core-contracts.sh v1.2.3

go get github.com/onflow/flow-core-contracts/lib/go/contracts@$1
go get github.com/onflow/flow-core-contracts/lib/go/templates@$1
cd integration
go get github.com/onflow/flow-core-contracts/lib/go/contracts@$1
go get github.com/onflow/flow-core-contracts/lib/go/templates@$1
cd ../insecure/
go get github.com/onflow/flow-core-contracts/lib/go/contracts@$1
go get github.com/onflow/flow-core-contracts/lib/go/templates@$1
cd ..
