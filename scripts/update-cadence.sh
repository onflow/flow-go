#!/bin/sh
#
# This script updates all cadence dependencies to a new version.
# Specify the desired version as the only argument when running the script:
#   ./scripts/update-cadence.sh v1.2.3

go get github.com/onflow/cadence@$1
cd integration
go get github.com/onflow/cadence@$1
cd ../insecure/
go get github.com/onflow/cadence@$1
cd ..
