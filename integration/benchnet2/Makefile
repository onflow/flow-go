DOCKER_TAG := $(shell git rev-parse --short HEAD)

# default values that can callers override when calling target
ACCESS_NODES=2
COLLECTION_NODES=6
CONSENSUS_NODES=3
EXECUTION_NODES=2
VERIFICATION_NODES=1

gen-bootstrap:
	go run -tags relic ../../cmd/bootstrap genconfig --address-format "%s%d.${NAMESPACE}:3569" --access $(ACCESS_NODES) --collection $(COLLECTION_NODES) --consensus $(CONSENSUS_NODES) --execution $(EXECUTION_NODES) --verification $(VERIFICATION_NODES) --weight 1000 -o ./ --config ./bootstrap/conf/node-config.json
	go run -tags relic ../../cmd/bootstrap keygen --machine-account --config ./bootstrap/conf/node-config.json -o ./bootstrap/keys
	echo {} > ./bootstrap/conf/partner-stakes.json
	mkdir ./bootstrap/partner-nodes
	go run -tags relic ../../cmd/bootstrap rootblock  --root-chain bench --root-height 0 --root-parent 0000000000000000000000000000000000000000000000000000000000000000 --config ./bootstrap/conf/node-config.json -o ./bootstrap/ --fast-kg --partner-dir ./bootstrap/partner-nodes --partner-weights ./bootstrap/conf/partner-stakes.json --internal-priv-dir ./bootstrap/keys/private-root-information
	go run -tags relic ../../cmd/bootstrap finalize --root-commit 0000000000000000000000000000000000000000000000000000000000000000 --service-account-public-key-json "{\"PublicKey\":\"tgiZNE8XebtMbfWoHbc+V4HYld8G3ulR6BPrqZ523TySiMzrWlqa2jkGcfYLcfP9JlPKXE4czG+LWmK+bRclag==\",\"SignAlgo\":2,\"HashAlgo\":1,\"SeqNumber\":0,\"Weight\":1000}" --config ./bootstrap/conf/node-config.json -o ./bootstrap/ --partner-dir ./bootstrap/partner-nodes --partner-weights ./bootstrap/conf/partner-stakes.json --collection-clusters 1 --epoch-counter 0 --epoch-length 1600 --epoch-staking-phase-length 50 --epoch-dkg-phase-length 500 --genesis-token-supply="1000000000.0" --protocol-version=0 --internal-priv-dir ./bootstrap/keys/private-root-information --dkg-data ./bootstrap/private-root-information/root-dkg-data.priv.json --root-block ./bootstrap/public-root-information/root-block.json --root-block-votes-dir ./bootstrap/public-root-information/root-block-votes/

# go run cmd/level1/bootstrap.go  --data "./testdata/level1/data/root-protocol-state-snapshot1.json"
gen-helm-l1:
	go run automate/cmd/level1/bootstrap.go --data bootstrap/public-root-information/root-protocol-state-snapshot.json --tag $(DOCKER_TAG)

# // go run cmd/level2/template.go --data "./testdata/level2/data/values8-verification-nodes-if-loop.json" --template "./testdata/level2/templates/values8-verification-nodes-if-loop.yml" --file=true
gen-helm-l2:
	go run automate/cmd/level2/template.go --data template-data.json --template automate/templates/helm-values-all-nodes.yml --file=true

# main target for creating dynamic helm values.yml chart
# runs bootstrap to generate all node info
# runs level 1 automation to read bootstrap data and generate data input for level 2
# runs level 2 automation to generate values.yml based on template and data values from previous step
gen-helm-values: gen-bootstrap gen-helm-l1 gen-helm-l2


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
