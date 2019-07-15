#!/bin/bash

godoc2md="go run github.com/lanre-ade/godoc2md"

$godoc2md github.com/dapperlabs/bamboo-node/internal/protocol/collect/clusters > internal/protocol/collect/clusters/README.md
$godoc2md github.com/dapperlabs/bamboo-node/internal/protocol/collect/routing > internal/protocol/collect/routing/README.md
$godoc2md github.com/dapperlabs/bamboo-node/internal/protocol/collect/collections > internal/protocol/collect/collections/README.md
$godoc2md github.com/dapperlabs/bamboo-node/data/keyvalue > data/keyvalue/README.md
