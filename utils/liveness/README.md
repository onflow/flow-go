# Liveness Checks Helper
For applications that need to perform liveness checks, this utility creates a simple object capable of checking in with multiple routines.

## Usage 
The suggested usage is to create a collector and then spawn checks from it to use in your application.

Each worker gets a single Check to work from, and the collector itself is registered as an HTTP handler to respond to liveness probes.

```go
multiCheck := liveness.NewCheckCollector(time.Second)

http.Handle("/live", multiCheck)

go worker1(multiCheck.NewCheck())
go worker2(multiCheck.NewCheck())

check3 = multiCheck.NewCheck()

for {
    check3.CheckIn()
    // do work
    // ...
}
```
