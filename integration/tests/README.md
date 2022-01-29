## How to debug if the integration tests failed

### Step 1 find out which tests is failing

Usually it's easy to tell from the error message. However, there might be errors not indicating clearly the name of the tests that were failing. This could be happen especially when the tests are flakey.

How do we know which test case is failing if the error isn't clear about it?

Using log.

Usually when tests are passing, no log will be printed. But when the tests is failing, _all_ logs for the failed tests in that package will be printed.

In order to use log, they must be printed in a way that is easy to reason about.

If logs are printed in pairs, one at the entrance and the other at the exit of a test case, then if we find both logs, it means the test case has passed; if we only find the entrance log line, but no log for the exit, it means the test case must have failed.

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

Why adding timestamp to the log?

Because golang will reorder logs if the tests were run without the `-v` flag. When `-v` is not specified, golang will cache the logs and print them only if the test case fails. However, the logs from docker container will not be reordered, this makes it hard to reason about with the logs. Adding timestamp is not the perfect solution to it, but at least provides data to reason about.
