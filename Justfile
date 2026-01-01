set shell := ["bash", "-lc"]

help:
    just -l

install:
    @echo "Installing Go deps + tools..."
    @go mod download
    @echo "Installing goimports..."
    @go install golang.org/x/tools/cmd/goimports@latest
    @echo "Installing golangci-lint..."
    @go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.0

fmt:
    @echo "Formatting..."
    @goimports -w .

vet:
    @echo "Vetting..."
    @go vet ./...

lint *targets:
    @echo "Running linters..."
    @if [ -z "{{targets}}" ]; then \
        golangci-lint run --config .golangci.yml ./...; \
    else \
        golangci-lint run --config .golangci.yml {{targets}}; \
    fi

check:
    @echo "Running code checks..."
    @echo ""
    @echo "Running goimports..."
    @goimports -w .
    @echo ""
    @echo "Running go mod tidy..."
    @go mod tidy
    @echo ""
    @echo "Running go build..."
    @go build ./...
    @echo ""
    @echo "Running linters..."
    @golangci-lint run --config .golangci.yml ./...
    @echo ""
    @echo "✓ All checks complete"

test *args:
    @echo "Running tests..."
    @if [ -z "{{args}}" ]; then \
        go test ./...; \
    else \
        go test {{args}}; \
    fi

test-one name *args:
    @echo "Running single test..."
    @go test ./... -run "{{name}}" {{args}}
