# Emulator

The Flow emulator emulates the Flow blockchain and the interface for interacting 
with it. 

## Use
The recommended way to use the emulator is as a Docker container. 

Docker builds for the emulator are automatically built and pushed to 
`gcr.io/dl-flow/emulator`, tagged by commit.

### Configuration
The emulator can be configured by setting environment variables when running 
the container. The configurable variables are specified [by the config of the `start` command](https://github.com/dapperlabs/flow-go/blob/master/internal/cli/emulator/start/start.go#L18-L24).
The environment variable names are the uppercased struct fields names, prefixed
by `FLOW_`.

For example, to run the emulator on port 9001 in verbose mode:
```bash
docker run -e FLOW_PORT=9001 -e FLOW_VERBOSE=true gcr.io/dl-flow/emulator
```

#### Accounts
The emulator uses a `flow.json` configuration file to store persistent
configuration, including account keys. In order to start, at least one
key (called the root key) must be configured. This key is used by default
when creating other accounts.

Because Docker does not persist files by default, this file will be 
re-generated each time the emulator starts when using Docker. For situations
where it is important that the emulator always uses the same root key (ie.
unit tests) you can specify a hex-encoded key as an environment variable.

```bash
docker run -e FLOW_ROOTKEY=<hex-encoded key> gcr.io/dl-flow/emulator
```

To generate a root key, use the `keys generate` command.
```bash
docker run gcr.io/dl-flow/emulator keys generate
```

## Building
To build the container locally, use `make docker-build-emulator`.

Images are automatically built and pushed to `gcr.io/dl-flow/emulator` via [Team City](https://ci.eng.dapperlabs.com/project/Flow_FlowGo_FlowEmulator) on any push to master, i.e. Pull Request merge

## Deployment
Currently, the emulator is being deployed via [Team City](https://ci.eng.dapperlabs.com/project/Flow_FlowGo_FlowEmulator)
All commands relating to the deployment process live in the `Makefile` in this directory

The deployment has a persistent volume and should keep state persistent. 
The deployment file for the Ingress/Service/Deployment combo, and the
Persistent Volume Claim are located in the k8s folder.

### Accessing the deployment

#### Web Address
*This is only available from the office currently*

It should be available publically via `https://grpc.emulator.staging.withflow.org/`

#### Port Forwarding
If you have direct access to the cluster, you can use `kubectl` to access the emulator
```bash
export EMULATOR_POD_NAME=$(kubectl get pod -n flow -l app=flow-emulator -o jsonpath="{.items[0].metadata.name}")
kubectl port-forward -n flow $EMULATOR_POD_NAME 3569:3569
```

#### From a Kubernetes Pod
If you are in the same namespace (flow), you can simply access the service via it's service name, e.g. `flow-emulator-v1`.
If you are in a different namespace, you will need to use a [Service without selectors](https://kubernetes.io/docs/concepts/services-networking/service/#services-without-selectors)
e.g.
```yaml
kind: Service
apiVersion: v1
metadata:
  name: flow-emulator-v1
  namespace: YOUR_NAMESPACE
spec:
  type: ExternalName
  externalName: flow-emulator-v1.flow.svc.cluster.local
  ports:
      protocol: TCP
      port: 3569
      targetPort: 3569
```
