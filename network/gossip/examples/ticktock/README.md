# TickTock
A simple pinger made with the gossip network layer.

## How to run
Go to `network/gossip/examples/ticktock`
and run the following:

### Build
```bash
go build -o ticktock .
```

### Run
Run three instances of the pinger
```
./ticktock #(in three different terminals, but one is enough)
```

Those instances will start to communicate with each other and display the time
(in seconds since the unix epoch)

```
The time is: 1569470575
...
```
