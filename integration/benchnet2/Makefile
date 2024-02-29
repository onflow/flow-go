# default value of the Docker base registry URL which can be overriden when invoking the Makefile
DOCKER_REGISTRY := us-west1-docker.pkg.dev/dl-flow-benchnet-automation/benchnet
CONFIGURATION_BUCKET := flow-benchnet-automation

# default values that callers can override when calling target
ACCESS = 1
COLLECTION = 2
VALID_COLLECTION := $(shell test $(COLLECTION) -ge 2; echo $$?)
CONSENSUS = 2
VALID_CONSENSUS := $(shell test $(CONSENSUS) -ge 2; echo $$?)
EXECUTION = 2
VALID_EXECUTION := $(shell test $(EXECUTION) -ge 2; echo $$?)
VERIFICATION = 1

validate:
ifeq ($(strip $(VALID_EXECUTION)), 1)
	# multiple execution nodes are required to prevent seals being generated in case of execution forking.
	$(error Number of Execution nodes should be no less than 2)
else ifeq ($(strip $(VALID_CONSENSUS)), 1)
	$(error Number of Consensus nodes should be no less than 2)
else ifeq ($(strip $(VALID_COLLECTION)), 1)
	$(error Number of Collection nodes should be no less than 2)
else ifeq ($(strip $(NETWORK_ID)),)
	$(error NETWORK_ID cannot be empty)
else ifeq ($(strip $(NAMESPACE)),)
	$(error NAMESPACE cannot be empty)
endif

# assumes there is a checked out version of flow-go in a "flow-go" sub-folder at this level so that the bootstrap executable
# for the checked out version will be run in the sub folder but the bootstrap folder will be created here (outside of the checked out flow-go in the sub folder)
gen-bootstrap: clone-flow
	cd flow-go && make crypto_setup_gopath
	cd flow-go/cmd/bootstrap && go run . genconfig --address-format "%s%d-${NETWORK_ID}.${NAMESPACE}:3569" --access $(ACCESS) --collection $(COLLECTION) --consensus $(CONSENSUS) --execution $(EXECUTION) --verification $(VERIFICATION) --weight 100 -o ./ --config ../../../bootstrap/conf/node-config.json
	cd flow-go/cmd/bootstrap && go run . keygen --machine-account --config ../../../bootstrap/conf/node-config.json -o ../../../bootstrap/keys
	echo {} > ./bootstrap/conf/partner-stakes.json
	mkdir ./bootstrap/partner-nodes
	cd flow-go/cmd/bootstrap && go run . rootblock  --root-chain bench --root-height 0 --root-parent 0000000000000000000000000000000000000000000000000000000000000000 --config ../../../bootstrap/conf/node-config.json -o ../../../bootstrap/ --partner-dir ../../../bootstrap/partner-nodes --partner-weights ../../../bootstrap/conf/partner-stakes.json --internal-priv-dir ../../../bootstrap/keys/private-root-information
	cd flow-go/cmd/bootstrap && go run . finalize --root-commit 0000000000000000000000000000000000000000000000000000000000000000 --service-account-public-key-json "{\"PublicKey\":\"R7MTEDdLclRLrj2MI1hcp4ucgRTpR15PCHAWLM5nks6Y3H7+PGkfZTP2di2jbITooWO4DD1yqaBSAVK8iQ6i0A==\",\"SignAlgo\":2,\"HashAlgo\":1,\"SeqNumber\":0,\"Weight\":1000}" --config ../../../bootstrap/conf/node-config.json -o ../../../bootstrap/ --partner-dir ../../../bootstrap/partner-nodes --partner-weights ../../../bootstrap/conf/partner-stakes.json --collection-clusters 1 --epoch-counter 0 --epoch-length 30000 --epoch-staking-phase-length 20000 --epoch-dkg-phase-length 2000 --genesis-token-supply="1000000000.0" --protocol-version=0 --internal-priv-dir ../../../bootstrap/keys/private-root-information --dkg-data ../../../bootstrap/private-root-information/root-dkg-data.priv.json --root-block ../../../bootstrap/public-root-information/root-block.json --root-block-votes-dir ../../../bootstrap/public-root-information/root-block-votes/ --epoch-commit-safety-threshold=1000

gen-helm-l1:
	go run automate/cmd/level1/bootstrap.go --data bootstrap/public-root-information/root-protocol-state-snapshot.json --dockerTag $(DOCKER_TAG) --dockerRegistry $(DOCKER_REGISTRY)

gen-helm-l2:
	go run automate/cmd/level2/template.go --data template-data.json --template automate/templates/helm-values-all-nodes.yml --outPath="./values.yml"

# main target for creating dynamic helm values.yml chart
# runs bootstrap to generate all node info
# runs level 1 automation to read bootstrap data and generate data input for level 2
# runs level 2 automation to generate values.yml based on template and data values from previous step
gen-helm-values: validate gen-bootstrap gen-helm-l1 gen-helm-l2

# main target for deployment
deploy-all: validate gen-helm-values k8s-secrets-create helm-deploy

# main target for cleaning up a deployment
clean-all: validate k8s-delete k8s-delete-secrets clean-bootstrap clean-gen-helm clean-flow

# target to be used in workflow as local clean up will not be needed
remote-clean-all: validate delete-configuration k8s-delete

clean-bootstrap:
	rm -rf ./bootstrap

clean-gen-helm:
	rm -f values.yml
	rm -f template-data.json

download-values-file:
	gsutil cp gs://${CONFIGURATION_BUCKET}/${NETWORK_ID}/values.yml .

upload-bootstrap:
	tar -cvf ${NETWORK_ID}.tar -C ./bootstrap .
	gsutil cp ${NETWORK_ID}.tar gs://${CONFIGURATION_BUCKET}/${NETWORK_ID}.tar
	gsutil cp values.yml gs://${CONFIGURATION_BUCKET}/${NETWORK_ID}/values.yml

helm-deploy: download-values-file
	helm upgrade --install -f ./values.yml ${NETWORK_ID} ./flow --set ingress.enabled=true --set networkId="${NETWORK_ID}" --set owner="${OWNER}" --set configurationBucket="${CONFIGURATION_BUCKET}" --debug --namespace ${NAMESPACE} --wait

k8s-delete:
	helm delete ${NETWORK_ID} --namespace ${NAMESPACE}
	kubectl delete pvc -l networkId=${NETWORK_ID} --namespace ${NAMESPACE}

delete-configuration:
	gsutil rm gs://${CONFIGURATION_BUCKET}/${NETWORK_ID}.tar
	gsutil rm gs://${CONFIGURATION_BUCKET}/${NETWORK_ID}/values.yml 

k8s-pod-health: validate
	kubectl get pods --namespace ${NAMESPACE}

clone-flow: clean-flow 
	# this cloned repo will be used for generating bootstrap info specific to that tag / version
	git clone https://github.com/onflow/flow-go.git
	cd flow-go && git checkout $(REF_FOR_BOOTSTRAP)
	
clean-flow:
	rm -rf flow-go
