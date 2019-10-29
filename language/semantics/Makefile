.PHONY: kompiled test clean

kompiled: bpl-kompiled/timestamp

bpl-kompiled/timestamp: bpl.k
	kompile bpl.k

test: kompiled
	tests/run_tests.sh

clean:
	rm -rf bpl-kompiled/
