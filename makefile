.PHONY: integration_tests

integration_tests:
	go test -v ./internal/tests/... -count=1