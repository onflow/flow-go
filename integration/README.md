## How does Flow Integration Tests work

The integration tests create a network locally with different node types running as docker instances. The docker instances are created and started in the test case by a testing utility called `testingdock`.

`testingdock` is a small testing utility which uses local Docker to run a network of nodes. With it, we are able to create, start, stop, and remove docker containers in test cases written in golang.

Uses `github.com/m4ksio/testingdock` which is a slightly enhanced fork of some other library.
See `tests/mvp_test.go` for example usage.

### Running tests

Since the test cases run docker instances as a network of nodes, we need to ensure the latest docker images for each node type have been built. The test cases will create docker instances using the `latest` tag for each image.

To ensure the latest docker images have been built, you can run:

```
make docker-native-build-access
make docker-native-build-collection
make docker-native-build-consensus
make docker-native-build-execution
make docker-native-build-verification
make docker-native-build-ghost
```

Or simply run `make docker-native-build-flow`

After images have been built, we can run the integration tests:
```
make integration-test
```

### Debugging tests with more logs
When debugging test cases, it's useful to see more logs. Turn on verbose logs with:
```
export VERBOSE=true
make integration-test
```

The `VERBOSE` envvar will output the logs from the test cases itself. However most of the logs from the docker images are still hidden. That's because the log level for the nodes is set to be either `FatalLevel` or `WarnLevel`.
To inspect more logs from the node itself, you will need to find the nodeConfig and change the log level into `InfoLevel`. For example, for the consensus node:

Change:
```
nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.WarnLevel), testnet.WithID(conID))
```

Into:
```
nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.InfoLevel), testnet.WithID(conID))
```

### Ghost node
You might notice that we introduced a node type called Ghost node in the integration tests.

This essentially allows you to create a mock for a certain node type, so that you can have access to all the messages this Ghost node received from other nodes, or control the Ghost node to send arbitary messages to other nodes.

For instance, in consensus integration tests, we could create a Ghost node as a mock for execution node, so that we can listen to the execution node's incoming messages to check whether it has received a block proposal from consensus node.

And we could construct an execution receipt and control the Ghost execution node to send the execution receipt to the consensus node.

Why not just launching a full execution node instance in the consensus integration tests instead of using Ghost node?

Because launching a full execution node in the consensus integration tests will be resource heavy. It also slows down the test case and might introduce other noises. Using a ghost node as a mock simplifes the test cases, and allows us to focus more on the specfic consensus related test cases to cover.

### Rebuild image when debugging
During test cases debugging, you might want to update some code. However, if you run `make integration-test` after updating the code, the new change will not be included, because the integration tests still use the old code from the docker image, which was built before adding the changes.

So you need to rebuild all the images by running `make docker-native-build-flow` again before re-running the integration tests.

Rebuilding all images takes quite some time, here is a shortcut:

If consensus's code was changed, then only consensus's image need to be rebuilt, so simply run `make docker-native-build-consensus` instead of rebuilding all the images.

### Organization

All integration test files live under `tests`. This is used to distinguish
between unit tests of testing utilities and integration tests for the network
in the Makefile.

### Load testing

To send random transactions, for example to load test a network, run `cd integration/localnet; make load`.

In order to build a docker container with the benchmarking binary, run `make docker-native-build-loader` from the root of this repository.
