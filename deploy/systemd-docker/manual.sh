#!/bin/bash
​
if [ $# -lt 2 ]
then
    echo "Usage : $0 node version"
    exit
fi
​
case "$1" in
​
    access_1)  echo "Starting access_1"
        docker rm access_1 > /dev/null 2>&1
        docker run --network="host" -v=${PWD}/genesis-infos:/genesis-infos -v=${PWD}/badger:/badger -d --name access_1 gcr.io/dl-flow/access:$2 \
		--nodeid=14fa4d16911229cfcddd29560c944f2f922ac1559717dce27cdf0aca8cf3f501 \
		--bootstrapdir=/genesis-infos \
		--datadir=/badger \
        --rpc-addr=10.138.0.5:9000 \
        --ingress-addr=10.138.0.6:9000 \
        --script-addr=10.138.0.3:9000
        ;;
    collection_1)  echo "Starting collection_1"
        docker rm collection_1 > /dev/null 2>&1
        docker run --network="host" -v=${PWD}/genesis-infos:/genesis-infos -v=${PWD}/badger:/badger -d --name collection_1 gcr.io/dl-flow/collection:$2 \
		--nodeid=9bbea0644b5e91ec66b4afddef7083e896da545d530ee52b85c78afa88301556 \
		--bootstrapdir=/genesis-infos \
        --datadir=/badger \
        --ingress-addr=10.138.0.6:9000 \
        --nclusters=1
        ;;
    collection_2)  echo "Starting collection_2"
        docker rm collection_2 > /dev/null 2>&1
        docker run --network="host" -v=${PWD}/genesis-infos:/genesis-infos -v=${PWD}/badger:/badger -d --name collection_2 gcr.io/dl-flow/collection:$2 \
		--nodeid=feb54673763fb43256b1b2d20631b5305ef332c5f97cd8ceb603cbff592d9160 \
		--bootstrapdir=/genesis-infos \
        --datadir=/badger \
        --ingress-addr=10.138.0.7:9000 \
        --nclusters=1
        ;;
    consensus_1)  echo "Starting consensus_1"
        docker rm consensus_1 > /dev/null 2>&1
        docker run --network="host" -v=${PWD}/genesis-infos:/genesis-infos -v=${PWD}/badger:/badger -d --name consensus_1 gcr.io/dl-flow/consensus:$2 \
		--nodeid=1c12c210890c7ad4f02967a11b25e3492fe61b45eb6352ba2f034b3f08506729 \
		--bootstrapdir=/genesis-infos \
        --datadir=/badger
        ;;
    consensus_2)  echo "Starting consensus_2"
        docker rm consensus_2 > /dev/null 2>&1
        docker run --network="host" -v=${PWD}/genesis-infos:/genesis-infos -v=${PWD}/badger:/badger -d --name consensus_2 gcr.io/dl-flow/consensus:$2 \
		--nodeid=c2c62caca4fec63219892f44e4bff62160b45906c8e6b1d1f6d35394003f9aa0 \
		--bootstrapdir=/genesis-infos \
        --datadir=/badger
        ;;
    consensus_3)  echo "Starting consensus_3"
        docker rm consensus_3 > /dev/null 2>&1
        docker run --network="host" -v=${PWD}/genesis-infos:/genesis-infos -v=${PWD}/badger:/badger -d --name consensus_3 gcr.io/dl-flow/consensus:$2 \
		--nodeid=eaca4eba6aef286c482db7418d0cef902dd979d65b652d11ef862abcb7fe1f1a \
		--bootstrapdir=/genesis-infos \
        --datadir=/badger
        ;;
    execution_1)  echo "Starting execution_1"
        docker rm execution_1 > /dev/null 2>&1
        docker run --network="host" -v=${PWD}/genesis-infos:/genesis-infos -v=${PWD}/badger:/badger -v=${PWD}/execution:/execution -d --name execution_1 gcr.io/dl-flow/execution:$2 \
		--nodeid=63be1380ac3bf89a1819cb75d7b50cedf15403f1cdc8e1ddc9250061b8ebf1b0 \
		--bootstrapdir=/genesis-infos \
        --datadir=/badger \
        --triedir=/execution \
        --rpc-addr=10.138.0.3:9000
        ;;
    verification_1)  echo "Starting verification_1"
        docker rm verification_1 > /dev/null 2>&1
        docker run --network="host" -v=${PWD}/genesis-infos:/genesis-infos -v=${PWD}/badger:/badger -d --name verification_1 gcr.io/dl-flow/verification:$2 \
		--nodeid=f69250f382bfb5acd601365ffebe2c519d66f8d83f237727b6cbc128be463fb2 \
		--bootstrapdir=/genesis-infos \
        --datadir=/badger \
        --alpha=1
        ;;
    *) echo "no such node"
    ;;
esac
