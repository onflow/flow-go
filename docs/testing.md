# Testing

## Test
```
$ ./test.sh
```
If iterating just on failed test, then we can do so without rebuilding the system:
```
$ docker-compose up --build --no-deps test
```
TODO: move to Makefile (remove also shell script)