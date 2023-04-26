#!/bin/bash

set -x
set -o pipefail

cd flow-go-public/integration/localnet

git fetch
git fetch --tags

while read -r branch_hash; do
    hash="${branch_hash##*:}"
    branch="${branch_hash%%:*}"

    git checkout "$branch" || continue
    git reset --hard "$hash"  || continue

    git log --oneline | head -1
    git describe

    make -C ../.. crypto_setup_gopath
    make stop
    rm -f docker-compose.nodes.yml
    sudo rm -rf data profiler trie
    make clean-data

    make -e COLLECTION=12 VERIFICATION=12 NCLUSTERS=12 LOGLEVEL=INFO bootstrap
#    make -e COLLECTION=12 VERIFICATION=12 NCLUSTERS=12 LOGLEVEL=INFO init
    sudo DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.nodes.yml build || continue
    sudo DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.nodes.yml up -d || continue

    sleep 30;
    go run -tags relic ../benchmark/cmd/ci -log-level debug -git-repo-path ../../ -tps-initial 800 -tps-min 1 -tps-max 1200 -duration 30m

    make stop
    docker system prune -a -f
done <../../../master.recent
