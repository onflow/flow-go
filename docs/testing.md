# Testing

## Test
Run:
```
$ ./test.sh
```
If iterating just on failed test, then we can do so without rebuilding the system:
```
$ docker-compose up --build --no-deps test
```
Cleanup:
```
$ docker-compose down
```
TODO: move to Makefile (remove also shell script)
