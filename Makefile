.PHONY: test test-cover build clean lint fmt vet run help test-manual test-clean test-interactive

# Default target
help:
	@echo "DotCor Development Commands:"
	@echo ""
	@echo "  make test          Run all tests"
	@echo "  make test-cover    Run tests with coverage report (opens in browser)"
	@echo "  make test-verbose  Run tests with verbose output"
	@echo "  make test-manual   Run full manual test in .manual-test/"
	@echo "  make test-interactive Enter interactive test mode in .manual-test/"
	@echo "  make test-copy     Copy ~/dotfiles to test environment"
	@echo "  make test-clean    Clean .manual-test/ directory"
	@echo "  make build         Build all packages"
	@echo "  make run           Build and run dotcor"
	@echo "  make clean         Clean build artifacts"
	@echo "  make lint          Run linter (golangci-lint)"
	@echo "  make fmt           Format all Go code"
	@echo "  make vet           Run go vet"
	@echo "  make deps          Download dependencies"
	@echo ""

# Run all tests
test:
	go test ./...

# Run tests with coverage report
test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

# Run tests with verbose output
test-verbose:
	go test ./... -v

# Build all packages
build:
	go build ./...

# Build and run
run:
	go run cmd/dotcor/main.go

# Clean build artifacts
clean:
	go clean
	rm -f coverage.out

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Format all Go code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Download dependencies
deps:
	go mod download
	go mod tidy

# Build the binary
binary:
	go build -o bin/dotcor cmd/dotcor/main.go

# Install to GOPATH/bin
install:
	go install ./cmd/dotcor

# Manual testing
test-manual:
	@./scripts/test-manual.sh full

test-interactive:
	@./scripts/test-manual.sh interactive

test-copy:
	@./scripts/test-manual.sh copy-dotfiles

test-clean:
	@./scripts/test-manual.sh clean
