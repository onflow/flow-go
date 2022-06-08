## How to debug if the integration tests failed

### Step 1 find out which tests is failing

Usually it's easy to tell from the error message. However, there might be errors not indicating clearly the name of the tests that were failing. This could be happen especially when the tests are flakey.

How do we know which test case is failing if the error isn't clear about it?

Using the log messages.

Usually when tests are passing, no logs will be printed. But when the tests are failing, _all_ logs for the failed tests in that package will be printed.

In order to use log messages to debug, they must be printed in a way that is easy to reason about.

It is useful if logs are printed in pairs at the start and at the end of a test case.  Then when debugging a test case you can search for the start and end log for that test case, any logs in between will be for that test case only.  If we only find the entrance log line, but no log for the exit, it means the test case must have failed.

In order to easily find the log for the entrance and exit of a test case, I'm using the following logs:

```

func TestXXX(t *testing.T) {
	t.Logf("%v ================> START TESTING %v", time.Now().UTC(), t.Name())

  ...

	t.Logf("%v ================> FINISH TESTING %v", time.Now().UTC(), t.Name())
}
```

This will print two logs with the timestamp. The timestamp is useful here, which I will explain later.

Sometimes the tests is defined in a test suite, then you can log them in SetupTest and TearDownTest:
```
func (s *Suite) TestYYY() {
  ...
}

func (s *Suite) SetupTest() {
  t := s.T()
	t.Logf("%v ================> START TESTING %v", time.Now().UTC(), t.Name())

  ...
}

func (s *Suite) TearDownTest() {
  ...

  t := s.T()
	t.Logf("%v ================> FINISH TESTING %v", time.Now().UTC(), t.Name())
}
```


The reason why we add a timestamp to the logs is because golang will reorder logs if the tests were run without the `-v` flag. When `-v` is not specified, golang will cache the logs and print them only if the test case fails. However, the logs from docker container will not be reordered, this makes it hard to reason about with the logs. Adding timestamp is not the perfect solution to it, but at least provides data to reason about.
