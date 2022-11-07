ACCESS_NODES=4
COLLECTION_NODES=6
CONSENSUS_NODE=3
EXECUTION_NODES=2
VERIFICATION_NODE=1

gen-bootstrap:
	go run -tags relic ../../cmd/bootstrap genconfig --address-format "%s%d.${NAMESPACE}:3569" --access $(ACCESS_NODES) --collection $(COLLECTION_NODES) --consensus $(CONSENSUS_NODE) --execution $(EXECUTION_NODES) --verification $(VERIFICATION_NODE) --weight 1000 -o ./ --config ./bootstrap/conf/node-config.json
	go run -tags relic ../../cmd/bootstrap keygen --machine-account --config ./bootstrap/conf/node-config.json -o ./bootstrap/keys
	echo {} > ./bootstrap/conf/partner-stakes.json
	mkdir ./bootstrap/partner-nodes
	go run -tags relic ../../cmd/bootstrap rootblock  --root-chain bench --root-height 0 --root-parent 0000000000000000000000000000000000000000000000000000000000000000 --config ./bootstrap/conf/node-config.json -o ./bootstrap/ --fast-kg --partner-dir ./bootstrap/partner-nodes --partner-weights ./bootstrap/conf/partner-stakes.json --internal-priv-dir ./bootstrap/keys/private-root-information
	go run -tags relic ../../cmd/bootstrap finalize --root-commit 0000000000000000000000000000000000000000000000000000000000000000 --service-account-public-key-json "{\"PublicKey\":\"tgiZNE8XebtMbfWoHbc+V4HYld8G3ulR6BPrqZ523TySiMzrWlqa2jkGcfYLcfP9JlPKXE4czG+LWmK+bRclag==\",\"SignAlgo\":2,\"HashAlgo\":1,\"SeqNumber\":0,\"Weight\":1000}" --config ./bootstrap/conf/node-config.json -o ./bootstrap/ --partner-dir ./bootstrap/partner-nodes --partner-weights ./bootstrap/conf/partner-stakes.json --collection-clusters 1 --epoch-counter 0 --epoch-length 1600 --epoch-staking-phase-length 50 --epoch-dkg-phase-length 500 --genesis-token-supply="1000000000.0" --protocol-version=0 --internal-priv-dir ./bootstrap/keys/private-root-information --dkg-data ./bootstrap/private-root-information/root-dkg-data.priv.json --root-block ./bootstrap/public-root-information/root-block.json --root-block-votes-dir ./bootstrap/public-root-information/root-block-votes/

k8s-secrets-create:
	bash ./create-secrets.sh

helm-deploy:
	helm upgrade --install -f ./values.yml ${NAMESPACE} ./flow --debug --namespace ${NAMESPACE}

k8s-delete:
	helm delete ${NAMESPACE} --namespace ${NAMESPACE}
	kubectl delete pvc -l service=flow --namespace ${NAMESPACE}

k8s-delete-secrets:
	kubectl delete secrets -l service=flow  --namespace ${NAMESPACE}

k8s-expose-locally:
	kubectl port-forward service/access1 9000:9000 --namespace ${NAMESPACE}

k8s-pod-health:
	kubectl get pods

k8s-test-network-accessibility:
	flow blocks get latest --host localhost:9000
	flow accounts create --network benchnet --key  e0ef5e52955e6542287db4528b3e8acc84a2c204eee9609f7c3120d1dac5a11b1bcb39677511db14354aa8c1a0ef62151220d97f015d49a8f0b78b653b570bfd --signer benchnet-account -f ./flow.json
