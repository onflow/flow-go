# Collecter
An example of a dummy distributed storage system built with the collector's
gnode registry for demonstration purposes.


## How to run
Go to `network/gossip/examples/collector`
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
./cli -key exists put # submits the key "test
./cli -key exists check # Checks if a key "test" has been submitted successfully
./cli -key deosnotexist check # Checks if a key "test" has been submitted before
```
