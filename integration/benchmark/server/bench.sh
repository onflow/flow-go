#!/bin/bash

set -x
set -o pipefail

# assumes flow-go was already cloned by user
cd flow-go/integration/localnet

git fetch
git fetch --tags

while read -r branch_hash; do
    echo "The current directory (start of loop) is $PWD"
    hash="${branch_hash##*:}"
    branch="${branch_hash%%:*}"

    git checkout "$branch" || continue
    git reset --hard "$hash"  || continue

    git log --oneline | head -1
    git describe

    echo "The current directory (middle of loop) is $PWD"
    make -C ../.. crypto_setup_gopath
    make stop
#    rm -f docker-compose.nodes.yml
    rm -rf data profiler trie
    make clean-data
    echo "The current directory (middle2 of loop) is $PWD"
    make -e COLLECTION=12 VERIFICATION=12 NCLUSTERS=12 LOGLEVEL=INFO bootstrap
#    make -e COLLECTION=12 VERIFICATION=12 NCLUSTERS=12 LOGLEVEL=INFO init
#    DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.nodes.yml build || continue
#    DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.nodes.yml up -d || continue

    echo "The current directory (middle3 of loop) is $PWD"
    sleep 30;
    go run -tags relic ../benchmark/cmd/ci -log-level debug -git-repo-path ../../ -tps-initial 800 -tps-min 1 -tps-max 1200 -duration 30m

    make stop
    docker system prune -a -f
    echo "The current directory (end of loop) is $PWD"
done </opt/master.recent
