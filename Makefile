.PHONY: test test-unit test-integration test-coverage clean

# Default test command runs unit tests
test: test-unit

# Run unit tests
test-unit:
	go test -v -short ./...

# Run integration tests
test-integration:
	go test -v -tags=integration ./...

# Run all tests (unit + integration)
test-all:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Clean up test artifacts
clean:
	rm -f coverage.out
	rm -f *.test
	rm -f *.prof 