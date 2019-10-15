# Collecter
An example of a dummy distributed storage system built with the collector's
gnode registry for demonstration purposes.


## How to run
Go to `pkg/network/gossip/v1/examples/collector`
and run the following:

### Build
```bash
go build -o serv ./server
go build -o cli  ./client
```

### Run
Run up to three instances of the server
```
./serv #(in three different terminals, but one is enough)
```

In another terminal, run the client
```
./cli -key test check # Checks if a key "test" has been submitted before
./cli -key test put # submits the key "test
./cli -key test check # Checks if a key "test" has been submitted successfully
```
